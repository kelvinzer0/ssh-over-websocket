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
	addr string
}

// NewServer creates a new server instance
func NewServer(addr string) *Server {
	return &Server{addr: addr}
}

// Run starts the HTTP server
func (s *Server) Run() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ssh", handleSSH)

	log.Printf("Server listening on %s", s.addr)
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
