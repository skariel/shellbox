package sshserver

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
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
	clients      *infra.AzureClients
	allocator    *infra.ResourceAllocator
	logger       *slog.Logger
}

// New creates a new SSH server instance
func New(port int, clients *infra.AzureClients) (*Server, error) {
	privateKey, _, err := sshutil.LoadKeyPair(infra.BastionSSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key pair: %w", err)
	}

	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create resource allocator
	resourceQueries := infra.NewResourceGraphQueries(
		clients.ResourceGraphClient,
		clients.SubscriptionID,
		clients.ResourceGroupName,
	)
	allocator := infra.NewResourceAllocator(clients, resourceQueries)

	return &Server{
		port:      port,
		clients:   clients,
		allocator: allocator,
		logger:    infra.NewLogger(),
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

// dialBoxAtIP establishes connection to the box at specified IP
func (s *Server) dialBoxAtIP(boxIP string) (*ssh.Client, error) {
	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", boxIP, infra.BoxSSHPort), s.boxSSHConfig)
}

// handleSCP handles SCP file transfer sessions
func (s *Server) handleSCP(_ gssh.Session) error {
	// TODO: For now, SCP will need resource allocation similar to shell sessions
	// This is a simplified version that will need enhancement
	s.logger.Warn("SCP not yet supported with dynamic allocation")
	return fmt.Errorf("SCP not yet supported with dynamic allocation")
}

func (s *Server) handleSession(sess gssh.Session) {
	if len(sess.Command()) > 0 && sess.Command()[0] == "scp" {
		if err := s.handleSCP(sess); err != nil {
			s.logger.Error("SCP error", "error", err)
			if err := sess.Exit(1); err != nil {
				s.logger.Error("Error during exit(1)", "error", err)
			}
			return
		}
		if err := sess.Exit(0); err != nil {
			s.logger.Error("Error during exit(0)", "error", err)
		}
		return
	}

	// Check if this is a command (non-interactive) session
	if len(sess.Command()) > 0 {
		s.handleCommandSession(sess)
		return
	}

	// Reject interactive sessions without commands
	helpMsg := `Interactive shell sessions require specifying a box name.

Usage:
  ssh shellbox.dev box <box_name>

Examples:
  ssh shellbox.dev box dev1
  ssh shellbox.dev spinup myproject

For more help:
  ssh shellbox.dev help
`
	if _, err := sess.Write([]byte(helpMsg)); err != nil {
		s.logger.Error("Error writing help message", "error", err)
	}
	if err := sess.Exit(1); err != nil {
		s.logger.Error("Error during exit(1)", "error", err)
	}
}

func (s *Server) handleShellSession(sess gssh.Session, boxName string) {
	if _, err := sess.Write([]byte("\n\nHI FROM SHELLBOX!\n\n")); err != nil {
		s.logger.Error("Error writing to SSH session", "error", err)
		return
	}

	// Generate session ID and user key hash for logging
	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	var userKeyHash string
	if publicKey := sess.PublicKey(); publicKey != nil {
		hash := sha256.Sum256(publicKey.Marshal())
		userKeyHash = hex.EncodeToString(hash[:])[:16]
	}

	// Log the box name
	s.logger.Info("starting shell session", "sessionID", sessionID, "userKeyHash", userKeyHash, "boxName", boxName)

	// Log session start event
	now := time.Now()
	sessionEvent := infra.EventLogEntity{
		PartitionKey: now.Format("2006-01-02"),
		RowKey:       fmt.Sprintf("%s_session_start", now.Format("20060102T150405")),
		Timestamp:    now,
		EventType:    "session_start",
		SessionID:    sessionID,
		UserKey:      userKeyHash,
		BoxID:        boxName,
		Details:      fmt.Sprintf(`{"remote_addr":"%s","box_name":"%s"}`, sess.RemoteAddr(), boxName),
	}
	if err := infra.WriteEventLog(context.Background(), s.clients, sessionEvent); err != nil {
		s.logger.Warn("Failed to log session start event", "error", err)
	}

	// Allocate resources for this user session
	s.logger.Info("allocating resources", "sessionID", sessionID)
	ctx := context.Background()
	resources, err := s.allocator.AllocateResourcesForUser(ctx, userKeyHash)
	if err != nil {
		s.logger.Error("Failed to allocate resources", "error", err, "sessionID", sessionID)
		fmt.Fprintf(sess.Stderr(), "Error allocating resources: %v\n", err)
		return
	}

	// Ensure cleanup on session end
	defer func() {
		s.logger.Info("releasing resources", "sessionID", sessionID)
		if err := s.allocator.ReleaseResources(ctx, resources.InstanceID, resources.VolumeID); err != nil {
			s.logger.Error("Failed to release resources", "error", err, "sessionID", sessionID)
		}
	}()

	// Connect to allocated instance
	client, err := s.dialBoxAtIP(resources.InstanceIP)
	if err != nil {
		s.logger.Error("Failed to connect to allocated instance", "error", err, "sessionID", sessionID)
		fmt.Fprintf(sess.Stderr(), "Error connecting to allocated instance: %v\n", err)
		return
	}
	defer client.Close()

	// Log successful resource allocation and connection
	connectEvent := infra.EventLogEntity{
		PartitionKey: now.Format("2006-01-02"),
		RowKey:       fmt.Sprintf("%s_resource_connect", time.Now().Format("20060102T150405")),
		Timestamp:    time.Now(),
		EventType:    "resource_connect",
		SessionID:    sessionID,
		UserKey:      userKeyHash,
		BoxID:        resources.InstanceID,
		Details:      fmt.Sprintf(`{"instanceIP":"%s","volumeID":"%s"}`, resources.InstanceIP, resources.VolumeID),
	}
	if err := infra.WriteEventLog(ctx, s.clients, connectEvent); err != nil {
		s.logger.Warn("Failed to log resource connection", "error", err)
	}

	boxSession, err := client.NewSession()
	if err != nil {
		s.logger.Error("Failed to create box session", "error", err)
		fmt.Fprintf(sess.Stderr(), "Error creating session: %v\n", err)
		return
	}
	defer boxSession.Close()

	if err := s.setupPty(sess, boxSession); err != nil {
		s.logger.Error("Failed to setup PTY", "error", err)
		return
	}

	if err := s.handleIO(sess, boxSession); err != nil {
		s.logger.Error("Failed to handle IO", "error", err)
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
					s.logger.Error("Failed to change window size", "error", err)
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

// handleCommandSession handles non-interactive command sessions
func (s *Server) handleCommandSession(sess gssh.Session) {
	// Create command context from session
	ctx := s.createCommandContext(sess)

	// Join command arguments into a single string
	cmdLine := strings.Join(sess.Command(), " ")

	// Parse the command using Cobra
	result := parseCommand(cmdLine)

	// Handle the command based on its action
	switch result.Action {
	case ActionSpinup:
		s.handleSpinupCommand(ctx, result, sess)
	case ActionBox:
		s.handleBoxCommand(ctx, result, sess)
	case ActionHelp:
		s.handleHelpCommand(ctx, result, sess)
	case ActionVersion:
		s.handleVersionCommand(ctx, result, sess)
	case ActionWhoami:
		s.handleWhoamiCommand(ctx, result, sess)
	case ActionError:
		// Send error message to user
		if _, err := sess.Write([]byte(result.Output + "\n")); err != nil {
			s.logger.Error("Error writing command error to session", "error", err)
		}
		if err := sess.Exit(result.ExitCode); err != nil {
			s.logger.Error("Error during exit", "error", err, "code", result.ExitCode)
		}
	default:
		// Unknown action
		msg := fmt.Sprintf("Unknown command action: %s\n", result.Action)
		if _, err := sess.Write([]byte(msg)); err != nil {
			s.logger.Error("Error writing unknown action message", "error", err)
		}
		if err := sess.Exit(1); err != nil {
			s.logger.Error("Error during exit(1)", "error", err)
		}
	}
}

// generateUserID creates a consistent 32-character user ID from a public key
func generateUserID(publicKey ssh.PublicKey) string {
	if publicKey == nil {
		return ""
	}
	hash := sha256.Sum256(publicKey.Marshal())
	hexHash := hex.EncodeToString(hash[:])
	return hexHash[:infra.UserIDLength]
}

// createCommandContext extracts context information from the SSH session
func (s *Server) createCommandContext(sess gssh.Session) CommandContext {
	userID := generateUserID(sess.PublicKey())

	return CommandContext{
		UserID:     userID,
		RemoteAddr: sess.RemoteAddr().String(),
		SessionID:  fmt.Sprintf("cmd_%d", time.Now().UnixNano()),
	}
}

// handleSpinupCommand handles the spinup command by calling existing shell session logic
func (s *Server) handleSpinupCommand(ctx CommandContext, result CommandResult, sess gssh.Session) {
	if len(result.Args) == 0 {
		if _, err := sess.Write([]byte("Error: box name required\n")); err != nil {
			s.logger.Error("Error writing spinup error", "error", err)
		}
		if err := sess.Exit(1); err != nil {
			s.logger.Error("Error during exit(1)", "error", err)
		}
		return
	}

	boxName := result.Args[0]
	s.logger.Info("Spinup command received", "user", ctx.UserID, "box", boxName)

	// TODO: Implement actual box creation/allocation logic here
	// For now, just simulate successful box creation

	successMsg := fmt.Sprintf("Box '%s' created successfully!\n\nTo connect to your box, use:\n  ssh -p 2222 %s@shellbox.dev\n", boxName, boxName)

	if _, err := sess.Write([]byte(successMsg)); err != nil {
		s.logger.Error("Error writing spinup success message", "error", err)
	}
	if err := sess.Exit(0); err != nil {
		s.logger.Error("Error during exit(0)", "error", err)
	}
}

// handleBoxCommand handles the box command to connect to an existing box
func (s *Server) handleBoxCommand(ctx CommandContext, result CommandResult, sess gssh.Session) {
	if len(result.Args) == 0 {
		if _, err := sess.Write([]byte("Error: box name required\n")); err != nil {
			s.logger.Error("Error writing box error", "error", err)
		}
		if err := sess.Exit(1); err != nil {
			s.logger.Error("Error during exit(1)", "error", err)
		}
		return
	}

	boxName := result.Args[0]
	s.logger.Info("Box command received", "user", ctx.UserID[:16], "box", boxName)

	// Call the shell session handler with box name
	s.handleShellSession(sess, boxName)
}

// handleHelpCommand handles the help command
func (s *Server) handleHelpCommand(_ CommandContext, _ CommandResult, sess gssh.Session) {
	helpText := `Shellbox Development Environment Manager

Available commands:
  spinup <box_name>    Create and start a development box
  box <box_name>       Connect to an existing development box
  help                 Show this help information  
  version              Show version information
  whoami               Show current user information

Examples:
  ssh shellbox.dev spinup dev1
  ssh shellbox.dev box dev1
  ssh shellbox.dev help
  ssh shellbox.dev whoami

For more information, visit https://shellbox.dev
`

	if _, err := sess.Write([]byte(helpText)); err != nil {
		s.logger.Error("Error writing help text", "error", err)
	}
	if err := sess.Exit(0); err != nil {
		s.logger.Error("Error during exit(0)", "error", err)
	}
}

// handleVersionCommand handles the version command
func (s *Server) handleVersionCommand(_ CommandContext, _ CommandResult, sess gssh.Session) {
	versionText := "Shellbox v1.0.0\n"

	if _, err := sess.Write([]byte(versionText)); err != nil {
		s.logger.Error("Error writing version text", "error", err)
	}
	if err := sess.Exit(0); err != nil {
		s.logger.Error("Error during exit(0)", "error", err)
	}
}

// handleWhoamiCommand handles the whoami command
func (s *Server) handleWhoamiCommand(ctx CommandContext, _ CommandResult, sess gssh.Session) {
	if ctx.UserID == "" {
		if _, err := sess.Write([]byte("No public key found\n")); err != nil {
			s.logger.Error("Error writing whoami error", "error", err)
		}
		if err := sess.Exit(1); err != nil {
			s.logger.Error("Error during exit(1)", "error", err)
		}
		return
	}

	// Encode the public key as base64 for readability
	encodedKey := base64.StdEncoding.EncodeToString([]byte(ctx.UserID))
	output := fmt.Sprintf("User ID: %s\nRemote Address: %s\n", encodedKey, ctx.RemoteAddr)

	if _, err := sess.Write([]byte(output)); err != nil {
		s.logger.Error("Error writing whoami output", "error", err)
	}
	if err := sess.Exit(0); err != nil {
		s.logger.Error("Error during exit(0)", "error", err)
	}
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

	s.logger.Info("Starting SSH server", "port", s.port)
	return server.ListenAndServe()
}
