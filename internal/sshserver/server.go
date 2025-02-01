package sshserver

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/gliderlabs/ssh"
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
			// Forward to box instead
			cmd := exec.Command("ssh",
				"-tt",
				"-o", "StrictHostKeyChecking=no",
				"-o", "PreferredAuthentications=password", // Add this
				"-o", "PubkeyAuthentication=no", // Add this
				"-p", "2222",
				"ubuntu@10.1.0.4")

			cmd.Stdin = s
			cmd.Stdout = s
			cmd.Stderr = s

			if err := cmd.Run(); err != nil {
				fmt.Fprintf(s.Stderr(), "Error connecting to box: %v\n", err)
			}
		},
	}

	log.Printf("Starting SSH server on port %d", s.port)
	return server.ListenAndServe()
}
