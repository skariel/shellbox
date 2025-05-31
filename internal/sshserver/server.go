package sshserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"shellbox/internal/infra"
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
	clients      *infra.AzureClients
}

// New creates a new SSH server instance
func New(port int, clients *infra.AzureClients) (*Server, error) {
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
		clients: clients,
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

// handleSCP handles SCP file transfer sessions
func (s *Server) handleSCP(sess gssh.Session) error {
	// Connect to box
	client, err := s.dialBox()
	if err != nil {
		log.Printf("Failed to connect to box: %v", err)
		return fmt.Errorf("error connecting to box: %w", err)
	}
	defer client.Close()

	// Create new session for SCP
	boxSession, err := client.NewSession()
	if err != nil {
		log.Printf("Failed to create box session: %v", err)
		return fmt.Errorf("error creating session: %w", err)
	}
	defer boxSession.Close()

	// Connect pipes - gliderlabs/ssh Session implements io.Reader/io.Writer directly
	boxSession.Stdin = sess
	boxSession.Stdout = sess
	boxSession.Stderr = sess.Stderr()

	// Execute the same SCP command on the box
	cmd := strings.Join(sess.Command(), " ")
	return boxSession.Run(cmd)
}

func (s *Server) handleSession(sess gssh.Session) {
	if len(sess.Command()) > 0 && sess.Command()[0] == "scp" {
		if err := s.handleSCP(sess); err != nil {
			log.Printf("SCP error: %v", err)
			if err := sess.Exit(1); err != nil {
				log.Printf("Error during exit(1): %v", err)
			}
			return
		}
		if err := sess.Exit(0); err != nil {
			log.Printf("Error during exit(0): %v", err)
		}
		return
	}

	s.handleShellSession(sess)
}

func (s *Server) handleShellSession(sess gssh.Session) {
	if _, err := sess.Write([]byte("\n\nHI FROM SHELLBOX!\n\n")); err != nil {
		log.Printf("Error writing to SSH session: %v", err)
		return
	}

	// Generate session ID and user key hash for logging
	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	var userKeyHash string
	if publicKey := sess.PublicKey(); publicKey != nil {
		hash := sha256.Sum256(publicKey.Marshal())
		userKeyHash = hex.EncodeToString(hash[:])[:16]
	}

	// Log session start event
	now := time.Now()
	sessionEvent := infra.EventLogEntity{
		PartitionKey: now.Format("2006-01-02"),
		RowKey:       fmt.Sprintf("%s_session_start", now.Format("20060102T150405")),
		Timestamp:    now,
		EventType:    "session_start",
		SessionID:    sessionID,
		UserKey:      userKeyHash,
		Details:      fmt.Sprintf(`{"remote_addr":"%s"}`, sess.RemoteAddr()),
	}
	if err := infra.WriteEventLog(context.Background(), s.clients, sessionEvent); err != nil {
		log.Printf("Failed to log session start event: %v", err)
	}

	client, err := s.dialBox()
	if err != nil {
		log.Printf("Failed to connect to box: %v", err)
		fmt.Fprintf(sess.Stderr(), "Error connecting to box: %v\n", err)

		// Log failed box connection
		failEvent := infra.EventLogEntity{
			PartitionKey: now.Format("2006-01-02"),
			RowKey:       fmt.Sprintf("%s_box_connect_fail", time.Now().Format("20060102T150405")),
			Timestamp:    time.Now(),
			EventType:    "box_connect_fail",
			SessionID:    sessionID,
			UserKey:      userKeyHash,
			Details:      fmt.Sprintf(`{"error":"%s"}`, err.Error()),
		}
		if err := infra.WriteEventLog(context.Background(), s.clients, failEvent); err != nil {
			log.Printf("Failed to log box connection failure: %v", err)
		}
		return
	}
	defer client.Close()

	// Log successful box connection
	connectEvent := infra.EventLogEntity{
		PartitionKey: now.Format("2006-01-02"),
		RowKey:       fmt.Sprintf("%s_box_connect", time.Now().Format("20060102T150405")),
		Timestamp:    time.Now(),
		EventType:    "box_connect",
		SessionID:    sessionID,
		UserKey:      userKeyHash,
		BoxID:        s.boxAddr, // Using box address as ID for now
		Details:      fmt.Sprintf(`{"box_addr":"%s"}`, s.boxAddr),
	}
	if err := infra.WriteEventLog(context.Background(), s.clients, connectEvent); err != nil {
		log.Printf("Failed to log box connection: %v", err)
	}

	boxSession, err := client.NewSession()
	if err != nil {
		log.Printf("Failed to create box session: %v", err)
		fmt.Fprintf(sess.Stderr(), "Error creating session: %v\n", err)
		return
	}
	defer boxSession.Close()

	if err := s.setupPty(sess, boxSession); err != nil {
		log.Printf("Failed to setup PTY: %v", err)
		return
	}

	if err := s.handleIO(sess, boxSession); err != nil {
		log.Printf("Failed to handle IO: %v", err)
	}
}

func (s *Server) setupPty(sess gssh.Session, boxSession *ssh.Session) error {
	if pty, winCh, isPty := sess.Pty(); isPty {
		if err := boxSession.RequestPty(pty.Term, pty.Window.Height, pty.Window.Width, ssh.TerminalModes{}); err != nil {
			return fmt.Errorf("failed to request PTY: %w", err)
		}

		go func() {
			for win := range winCh {
				if err := boxSession.WindowChange(win.Height, win.Width); err != nil {
					log.Printf("Failed to change window size: %v", err)
				}
			}
		}()
	}
	return nil
}

func (s *Server) handleIO(sess gssh.Session, boxSession *ssh.Session) error {
	stdin, err := boxSession.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := boxSession.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := boxSession.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := boxSession.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

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

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error copying data: %w", err)
	}

	if err := boxSession.Wait(); err != nil {
		return fmt.Errorf("session ended with error: %w", err)
	}

	return nil
}

// Run starts the SSH server
func (s *Server) Run() error {
	server := gssh.Server{
		Addr: fmt.Sprintf(":%d", s.port),
		PublicKeyHandler: func(_ gssh.Context, _ gssh.PublicKey) bool {
			// Accept any key
			return true
		},
		Handler: s.handleSession,
	}

	log.Printf("Starting SSH server on port %d", s.port)
	return server.ListenAndServe()
}
