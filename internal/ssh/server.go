package ssh

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	"annet-oil/internal/annet"
	"annet-oil/internal/config"
	"annet-oil/internal/router"
)

type Server struct {
	config       *config.Config
	annetService *annet.Service
	router       *router.Router
	listener     net.Listener
	mu           sync.Mutex
	stopped      bool
}

func NewServer(cfg *config.Config, annetService *annet.Service, router *router.Router) (*Server, error) {
	return &Server{
		config:       cfg,
		annetService: annetService,
		router:       router,
	}, nil
}

func (s *Server) Start(addr string) error {
	sshConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}

	hostKey, err := s.getHostKey()
	if err != nil {
		return fmt.Errorf("failed to get host key: %w", err)
	}

	sshConfig.AddHostKey(hostKey)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	log.Printf("SSH server started on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.mu.Lock()
			stopped := s.stopped
			s.mu.Unlock()
			if stopped {
				return nil
			}
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go s.handleConnection(conn, sshConfig)
	}
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *Server) handleConnection(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()

	sshConn, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		log.Printf("Failed to handshake: %v", err)
		return
	}
	defer sshConn.Close()

	log.Printf("New SSH connection from %s", sshConn.RemoteAddr())

	go ssh.DiscardRequests(requests)

	for channel := range channels {
		if channel.ChannelType() != "session" {
			channel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		go s.handleSession(channel)
	}
}

func (s *Server) handleSession(newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Failed to accept channel: %v", err)
		return
	}
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "exec":
			if len(req.Payload) < 4 {
				req.Reply(false, nil)
				continue
			}

			commandLen := uint32(req.Payload[0])<<24 | uint32(req.Payload[1])<<16 | uint32(req.Payload[2])<<8 | uint32(req.Payload[3])
			if len(req.Payload) < int(4+commandLen) {
				req.Reply(false, nil)
				continue
			}

			command := string(req.Payload[4 : 4+commandLen])
			req.Reply(true, nil)

			exitCode := s.executeCommand(channel, command)

			if exitCode == 0 {
				channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			} else {
				channel.SendRequest("exit-status", false, []byte{0, 0, 0, byte(exitCode)})
			}

		case "shell":
			req.Reply(true, nil)
			s.handleShell(channel)

		default:
			req.Reply(false, nil)
		}
	}
}

func (s *Server) executeCommand(channel ssh.Channel, command string) int {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		fmt.Fprintln(channel, "No command specified")
		return 1
	}

	if parts[0] == "annet-oil" {
		return s.executeAnnetOilCommand(channel, parts[1:])
	}

	fmt.Fprintf(channel, "Command not supported: %s\n", parts[0])
	fmt.Fprintln(channel, "Available commands: annet-oil")
	return 1
}

func (s *Server) executeAnnetOilCommand(channel ssh.Channel, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(channel, "Usage: annet-oil <command> [options]")
		fmt.Fprintln(channel, "Available commands: gen, diff, patch, deploy, containers, routing")
		return 1
	}

	cmdArgs := []string{"annet-oil"}
	cmdArgs = append(cmdArgs, args...)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(channel, "Error setting up command: %v\n", err)
		return 1
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(channel, "Error setting up command: %v\n", err)
		return 1
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(channel, "Error starting command: %v\n", err)
		return 1
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(channel, stdout)
	}()

	go func() {
		defer wg.Done()
		io.Copy(channel, stderr)
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		return 1
	}

	return 0
}

func (s *Server) handleShell(channel ssh.Channel) {
	fmt.Fprintln(channel, "Annet Oil SSH Shell")
	fmt.Fprintln(channel, "Available commands: annet-oil")
	fmt.Fprintln(channel, "Type 'exit' to close the connection")

	for {
		fmt.Fprint(channel, "annet-oil> ")

		var command string
		fmt.Fscanf(channel, "%s\n", &command)

		if command == "exit" {
			fmt.Fprintln(channel, "Goodbye!")
			return
		}

		if command == "" {
			continue
		}

		s.executeCommand(channel, command)
	}
}

func (s *Server) getHostKey() (ssh.Signer, error) {
	keyPath := "/tmp/annet-oil-host-key"

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		if err := s.generateHostKey(keyPath); err != nil {
			return nil, fmt.Errorf("failed to generate host key: %w", err)
		}
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read host key: %w", err)
	}

	key, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host key: %w", err)
	}

	return key, nil
}

func (s *Server) generateHostKey(keyPath string) error {
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return err
	}

	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", keyPath, "-N", "")
	return cmd.Run()
}