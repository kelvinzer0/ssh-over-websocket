package gateway

import (
	"embed"
	"log"
	"net/http"
)

//go:embed index.html
var staticFiles embed.FS

// Server holds dependencies for the gateway
type Server struct {
	addr     string
	certFile string
	keyFile  string
}

// NewServer creates a new server instance
func NewServer(addr, certFile, keyFile string) *Server {
	return &Server{addr: addr, certFile: certFile, keyFile: keyFile}
}

// Run starts the HTTP(S) server
func (s *Server) Run() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ssh", handleSSH)

	if s.certFile != "" && s.keyFile != "" {
		log.Printf("Server listening on HTTPS %s", s.addr)
		return http.ListenAndServeTLS(s.addr, s.certFile, s.keyFile, mux)
	}

	log.Printf("Server listening on HTTP %s", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	content, err := staticFiles.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Could not read index.html", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(content)
}
