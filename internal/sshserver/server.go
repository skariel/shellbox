package sshserver

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

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

			sshArgs := []string{
				"-o", "StrictHostKeyChecking=no",
				"-o", "SendEnv=TERM",
				"-p", "2222",
			}

			// Only force TTY allocation if client requested PTY
			if pty, winCh, isPty := s.Pty(); isPty {
				sshArgs = append(sshArgs, "-tt")
				cmd := exec.Command("ssh", append(sshArgs, "ubuntu@10.1.0.4")...)

				// Set up environment with terminal type and size
				cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", pty.Term))
				cmd.Env = append(os.Environ(), 
					"SSH_TTY=/dev/pts/0",
					fmt.Sprintf("LINES=%d", pty.Window.Height),
					fmt.Sprintf("COLUMNS=%d", pty.Window.Width))

				cmd.Stdin = s
				cmd.Stdout = s
				cmd.Stderr = s

				// Start command
				if err := cmd.Start(); err != nil {
					log.Printf("Failed to start command: %v", err)
					return
				}

				// Handle window size changes using kill -SIGWINCH
				go func() {
					for win := range winCh {
						// Get the PID of the remote sshd process
						pidCmd := exec.Command("ssh",
							"-o", "StrictHostKeyChecking=no",
							"-p", "2222",
							"ubuntu@10.1.0.4",
							"ps -ef | grep sshd | grep pts | awk '{print $2}' | head -n1")
						pidBytes, err := pidCmd.Output()
						if err != nil {
							log.Printf("Failed to get remote PID: %v", err)
							continue
						}
						pid := strings.TrimSpace(string(pidBytes))
						
						// Send SIGWINCH to the remote process
						cmd := exec.Command("ssh",
							"-o", "StrictHostKeyChecking=no",
							"-p", "2222",
							"ubuntu@10.1.0.4",
							fmt.Sprintf("kill -SIGWINCH %s", pid))
						if err := cmd.Run(); err != nil {
							log.Printf("Failed to send SIGWINCH: %v", err)
						}
					}
				}()

				if err := cmd.Wait(); err != nil {
					fmt.Fprintf(s.Stderr(), "Error connecting to box: %v\n", err)
				}
			} else {
				// Non-PTY session
				cmd := exec.Command("ssh", append(sshArgs, "ubuntu@10.1.0.4")...)
				cmd.Stdin = s
				cmd.Stdout = s
				cmd.Stderr = s

				if err := cmd.Run(); err != nil {
					fmt.Fprintf(s.Stderr(), "Error connecting to box: %v\n", err)
				}
			}
		},
	}

	log.Printf("Starting SSH server on port %d", s.port)
	return server.ListenAndServe()
}
