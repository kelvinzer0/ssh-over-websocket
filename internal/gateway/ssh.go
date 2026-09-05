package gateway

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/crypto/ssh"
)

func handleSSH(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	port := r.URL.Query().Get("port")
	user := r.URL.Query().Get("user")
	pass := r.URL.Query().Get("pass")

	if port == "" {
		port = "22"
	}

	if host == "" || user == "" || pass == "" {
		http.Error(w, "Missing required parameters (host, user, pass)", http.StatusBadRequest)
		return
	}

	log.Printf("[ws] new connection from %s → %s@%s:%s", r.RemoteAddr, user, host, port)

	// 1. Upgrade to WebSocket
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("[ws] upgrade failed: %v", err)
		return
	}
	defer conn.CloseNow()

	// Root context with cancel — cancels everything when this handler returns
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 2. SSH Connection Setup
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		log.Printf("[ssh] dial %s failed: %v", addr, err)
		conn.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf("\r\nFailed to connect to SSH server: %v\r\n", err)))
		conn.Close(websocket.StatusInternalError, "SSH connection failed")
		return
	}
	defer sshClient.Close()
	log.Printf("[ssh] connected to %s", addr)

	// 3. Open SSH Session
	session, err := sshClient.NewSession()
	if err != nil {
		log.Printf("[ssh] session failed: %v", err)
		conn.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf("\r\nFailed to create SSH session: %v\r\n", err)))
		conn.Close(websocket.StatusInternalError, "SSH session failed")
		return
	}
	defer session.Close()

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	colsStr := r.URL.Query().Get("cols")
	rowsStr := r.URL.Query().Get("rows")
	cols, _ := strconv.Atoi(colsStr)
	rows, _ := strconv.Atoi(rowsStr)
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		conn.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf("\r\nFailed to request PTY: %v\r\n", err)))
		conn.Close(websocket.StatusInternalError, "PTY failed")
		return
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return
	}

	// Start the default login shell
	if err := session.Shell(); err != nil {
		conn.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf("\r\nFailed to start shell: %v\r\n", err)))
		conn.Close(websocket.StatusInternalError, "Shell failed")
		return
	}

	// 4. WebSocket ↔ SSH Bridge
	// Use sync.WaitGroup to track all bridge goroutines
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Helper: close SSH session when any bridge goroutine exits
	closeSession := sync.OnceFunc(func() {
		log.Printf("[bridge] closing SSH session for %s@%s", user, host)
		session.Close()
		cancel()
	})

	// SSH stdout → WebSocket (binary to support all terminal bytes)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer closeSession()
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("[ssh] stdout read error: %v", err)
				}
				return
			}
			if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
				log.Printf("[ws] write error (stdout): %v", err)
				return
			}
		}
	}()

	// SSH stderr → WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
				return
			}
		}
	}()

	// WebSocket → SSH stdin
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer closeSession()
		defer close(done)
		for {
			_, p, err := conn.Read(ctx)
			if err != nil {
				log.Printf("[ws] read error: %v", err)
				return
			}
			if _, err := stdin.Write(p); err != nil {
				log.Printf("[ssh] stdin write error: %v", err)
				return
			}
		}
	}()

	// Keep-alive: ping every 15s, auto-recover on pong timeout
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
				err := conn.Ping(pingCtx)
				pingCancel()
				if err != nil {
					log.Printf("[ws] ping failed: %v", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for the read-side to finish (WebSocket→stdin closes done)
	<-done

	// Wait for all bridge goroutines to drain before closing WebSocket
	wg.Wait()
	log.Printf("[bridge] session ended for %s@%s", user, host)
	conn.Close(websocket.StatusNormalClosure, "SSH session ended")
}
