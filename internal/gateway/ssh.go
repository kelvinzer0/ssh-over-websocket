package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

	// 1. Upgrade to WebSocket (coder/websocket - concurrent-safe, auto ping/pong)
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow all origins
	})
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.CloseNow()

	// Root context with cancel - cancels everything when this handler returns
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 2. SSH Connection Setup
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		conn.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf("\r\nFailed to connect to SSH server: %v\r\n", err)))
		conn.Close(websocket.StatusInternalError, "SSH connection failed")
		return
	}
	defer sshClient.Close()

	// 3. Open SSH Session
	session, err := sshClient.NewSession()
	if err != nil {
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

	// 4. WebSocket - SSH Bridge
	done := make(chan struct{})

	// SSH stdout → WebSocket (binary to support all terminal bytes)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				break
			}
			if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
				break
			}
		}
		cancel()
	}()

	// SSH stderr → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				break
			}
			if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
				break
			}
		}
	}()

	// WebSocket → SSH stdin
	go func() {
		defer close(done)
		for {
			_, p, err := conn.Read(ctx)
			if err != nil {
				break
			}
			if _, err := stdin.Write(p); err != nil {
				break
			}
		}
		session.Close()
	}()

	// Keep-alive: ping every 20s, coder/websocket handles pong automatically
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
				err := conn.Ping(pingCtx)
				pingCancel()
				if err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	<-done
	conn.Close(websocket.StatusNormalClosure, "SSH session ended")
}
