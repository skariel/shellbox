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
			if _, err := s.Write([]byte("\n\nHI FROM SHELLBOX!\n\n")); err != nil {
				log.Printf("Error writing to SSH session: %v", err)
				return
			}

			cmd := exec.Command("ssh",
				"-tt",
				"-o", "StrictHostKeyChecking=no",
				"-o", "SendEnv=TERM",
				"-p", "2222",
				"ubuntu@10.1.0.4")

			cmd.Stdin = s
			cmd.Stdout = s
			cmd.Stderr = s

			// Get terminal info if PTY was requested
			if pty, winCh, isPty := s.Pty(); isPty {
				cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", pty.Term))
				go func() {
					for win := range winCh {
						// Could handle window size changes here if needed
						_ = win
					}
				}()
			}

			if err := cmd.Run(); err != nil {
				fmt.Fprintf(s.Stderr(), "Error connecting to box: %v\n", err)
			}
		},
	}

	log.Printf("Starting SSH server on port %d", s.port)
	return server.ListenAndServe()
}
