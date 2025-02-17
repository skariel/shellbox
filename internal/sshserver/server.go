package sshserver

import (
	"fmt"
	"io"
	"log"
	"time"

	"shellbox/internal/sshutil"

	gssh "github.com/gliderlabs/ssh" // alias to avoid confusion with crypto/ssh
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
)

// Server represents the SSH server configuration
type Server struct {
	port         int
	boxSSHConfig *ssh.ClientConfig
	boxAddr      string
}

// New creates a new SSH server instance
func New(port int) (*Server, error) {
	privateKey, _, err := sshutil.LoadKeyPair("$HOME/.ssh/id_rsa")
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key pair: %w", err)
	}

	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &Server{
		port:    port,
		boxAddr: "10.1.0.4",
		boxSSHConfig: &ssh.ClientConfig{
			User: "ubuntu",
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			// #nosec G106 -- Intentionally skipping host key verification because:
			// 1. Boxes are ephemeral with dynamic IPs and host keys
			// 2. Connections are within Azure VNet with strict NSG rules
			// 3. Network architecture prevents MITM attacks (see network.txt)
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         10 * time.Second,
		},
	}, nil
}

// dialBox establishes connection to the box
func (s *Server) dialBox() (*ssh.Client, error) {
	return ssh.Dial("tcp", fmt.Sprintf("%s:2222", s.boxAddr), s.boxSSHConfig)
}

// Run starts the SSH server
func (s *Server) Run() error {
	server := gssh.Server{
		Addr: fmt.Sprintf(":%d", s.port),
		PublicKeyHandler: func(_ gssh.Context, _ gssh.PublicKey) bool {
			// Accept any key
			return true
		},
		Handler: func(sess gssh.Session) {
			if _, err := sess.Write([]byte("\n\nHI FROM SHELLBOX!\n\n")); err != nil {
				log.Printf("Error writing to SSH session: %v", err)
				return
			}

			client, err := s.dialBox()
			if err != nil {
				log.Printf("Failed to connect to box: %v", err)
				fmt.Fprintf(sess.Stderr(), "Error connecting to box: %v\n", err)
				return
			}
			defer client.Close()

			boxSession, err := client.NewSession()
			if err != nil {
				log.Printf("Failed to create box session: %v", err)
				fmt.Fprintf(sess.Stderr(), "Error creating session: %v\n", err)
				return
			}
			defer boxSession.Close()

			// Handle PTY
			if pty, winCh, isPty := sess.Pty(); isPty {
				// Request PTY on the box session
				if err := boxSession.RequestPty(pty.Term, pty.Window.Height, pty.Window.Width, ssh.TerminalModes{}); err != nil {
					log.Printf("Failed to request PTY: %v", err)
					return
				}

				// Handle window size changes
				go func() {
					for win := range winCh {
						if err := boxSession.WindowChange(win.Height, win.Width); err != nil {
							log.Printf("Failed to change window size: %v", err)
						}
					}
				}()
			}

			// Set up pipes
			stdin, err := boxSession.StdinPipe()
			if err != nil {
				log.Printf("Failed to get stdin pipe: %v", err)
				return
			}
			stdout, err := boxSession.StdoutPipe()
			if err != nil {
				log.Printf("Failed to get stdout pipe: %v", err)
				return
			}
			stderr, err := boxSession.StderrPipe()
			if err != nil {
				log.Printf("Failed to get stderr pipe: %v", err)
				return
			}

			// Start shell
			if err := boxSession.Shell(); err != nil {
				log.Printf("Failed to start shell: %v", err)
				return
			}

			// Copy data in both directions using errgroup
			var g errgroup.Group
			g.Go(func() error {
				_, err := io.Copy(stdin, sess)
				return err
			})
			g.Go(func() error {
				_, err := io.Copy(sess, stdout)
				return err
			})
			g.Go(func() error {
				_, err := io.Copy(sess.Stderr(), stderr)
				return err
			})

			// Wait for copies to complete
			if err := g.Wait(); err != nil {
				log.Printf("Error copying data: %v", err)
			}

			// Wait for session to complete
			if err := boxSession.Wait(); err != nil {
				log.Printf("Session ended with error: %v", err)
			}
		},
	}

	log.Printf("Starting SSH server on port %d", s.port)
	return server.ListenAndServe()
}
