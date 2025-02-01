package sshserver

import (
	"fmt"
	"log"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// Server represents the SSH server configuration
type Server struct {
	port int
}

// New creates a new SSH server instance
func New(port int) *Server {
	return &Server{port: port}
}

// Run starts the SSH server
func (s *Server) Run() error {
	server := ssh.Server{
		Addr: fmt.Sprintf(":%d", s.port),
		PublicKeyHandler: func(_ ssh.Context, _ ssh.PublicKey) bool {
			// Accept any key
			return true
		},
		Handler: func(s ssh.Session) {
			// Print fingerprint and disconnect
			fmt.Fprintf(s, "Your key fingerprint: %s\n", gossh.FingerprintSHA256(s.PublicKey()))
			s.Close()
		},
	}

	log.Printf("Starting SSH server on port %d", s.port)
	return server.ListenAndServe()
}
