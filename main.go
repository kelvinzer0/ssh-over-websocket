package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

//go:embed index.html
var staticFiles embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for simplicity, in production this should be restricted
	},
}

// GatewayServer holds dependencies for the gateway
type GatewayServer struct {
	addr string
}

// NewGatewayServer creates a new server instance
func NewGatewayServer(addr string) *GatewayServer {
	return &GatewayServer{addr: addr}
}

func (s *GatewayServer) Run() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ssh", handleSSH)

	log.Printf("Server listening on %s", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *GatewayServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	content, err := staticFiles.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Could not read index.html", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(content)
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

	if err := session.Start("bash"); err != nil {
		// Fallback to default shell if bash fails
		if err := session.Shell(); err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nFailed to start shell: %v\r\n", err)))
			return
		}
	}

	// 4. WebSocket - SSH Bridge
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				break
			}
			conn.WriteMessage(websocket.TextMessage, buf[:n])
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
			conn.WriteMessage(websocket.TextMessage, buf[:n])
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

func installService(bindAddr string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %v", err)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=SSH over WebSocket Gateway
After=network.target

[Service]
ExecStart=%s -bind %s
Restart=always
User=root

[Install]
WantedBy=multi-user.target
`, exePath, bindAddr)

	servicePath := "/etc/systemd/system/ssh-gateway.service"
	err = os.WriteFile(servicePath, []byte(serviceContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write systemd service file: %v", err)
	}

	fmt.Println("Reloading systemd daemon...")
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload daemon: %v", err)
	}

	fmt.Println("Enabling and starting ssh-gateway service...")
	if err := exec.Command("systemctl", "enable", "--now", "ssh-gateway").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %v", err)
	}

	return nil
}

func main() {
	bind := flag.String("bind", ":8080", "Bind address (e.g., 127.0.0.1:8080 or :8080)")
	install := flag.Bool("install", false, "Install as a Linux systemd service")
	flag.Parse()

	if *install {
		if err := installService(*bind); err != nil {
			log.Fatalf("Install failed (make sure you run as root): %v", err)
		}
		fmt.Println("Service installed and started successfully. You can check status with: systemctl status ssh-gateway")
		return
	}

	server := NewGatewayServer(*bind)
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}
