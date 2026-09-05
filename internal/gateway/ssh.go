package gateway

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for simplicity, in production this should be restricted
	},
}

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

	// 1. Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

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
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nFailed to connect to SSH server: %v\r\n", err)))
		return
	}
	defer sshClient.Close()

	// 3. Open SSH Session
	session, err := sshClient.NewSession()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nFailed to create SSH session: %v\r\n", err)))
		return
	}
	defer session.Close()

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // enable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
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

	if err := session.RequestPty("xterm", cols, rows, modes); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nFailed to request PTY: %v\r\n", err)))
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

	// Start the default login shell (correct approach after RequestPty)
	if err := session.Shell(); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nFailed to start shell: %v\r\n", err)))
		return
	}

	// 4. WebSocket - SSH Bridge
	var wg sync.WaitGroup
	wg.Add(2)

	var writeMutex sync.Mutex
	writeWs := func(msgType int, data []byte) error {
		writeMutex.Lock()
		defer writeMutex.Unlock()
		return conn.WriteMessage(msgType, data)
	}

	// Server-side keep-alive (ping every 10 seconds)
	// to prevent reverse proxy idle timeouts
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := writeWs(websocket.PingMessage, nil); err != nil {
				break
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				break
			}
			if err := writeWs(websocket.TextMessage, buf[:n]); err != nil {
				break
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				break
			}
			if err := writeWs(websocket.TextMessage, buf[:n]); err != nil {
				break
			}
		}
	}()

	go func() {
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if messageType == websocket.TextMessage {
				stdin.Write(p)
			}
		}
		session.Close()
	}()

	wg.Wait()
}
