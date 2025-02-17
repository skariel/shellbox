package sshserver

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/gliderlabs/ssh"
	pssh "golang.org/x/crypto/ssh"
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
			s.Write([]byte("\n\nHI FROM SHELLBOX!\n\n"))

			// Get public key from session
			pubKey := s.PublicKey()
			if pubKey == nil {
				fmt.Fprintf(s.Stderr(), "No public key provided\n")
				return
			}

			// Convert to authorized_keys format using standard ssh package
			authKey := string(pssh.MarshalAuthorizedKey(pubKey))

			// Create command to append key to authorized_keys
			setupCmd := fmt.Sprintf(`mkdir -p ~/.ssh && echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys && chmod 700 ~/.ssh`, authKey)

			// Execute on the running VM
			keyCmd := exec.Command("ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "PreferredAuthentications=password",
				"-o", "PubkeyAuthentication=no",
				"-p", "2222",
				"ubuntu@10.1.0.4",
				setupCmd)

			if err := keyCmd.Run(); err != nil {
				fmt.Fprintf(s.Stderr(), "Error adding public key: %v\n", err)
				return
			}

			// Now proceed with main SSH connection, but without password restrictions
			cmd := exec.Command("ssh",
				"-tt",
				"-o", "StrictHostKeyChecking=no",
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
