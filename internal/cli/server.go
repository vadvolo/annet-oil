package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"annet-oil/internal/api"
	"annet-oil/internal/gnetcli"
	"annet-oil/internal/ssh"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server management commands",
	Long:  `Commands for managing annet-oil servers`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start API and SSH servers",
	Long:  `Start both API and SSH servers`,
	RunE:  runServerStartCommand,
}

var serverApiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start only API server",
	Long:  `Start only the REST API server`,
	RunE:  runAPIServerCommand,
}

var serverSshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "Start only SSH server",
	Long:  `Start only the SSH server`,
	RunE:  runSSHServerCommand,
}

func init() {
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverApiCmd)
	serverCmd.AddCommand(serverSshCmd)
}

func runServerStartCommand(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := startAPIServer(ctx); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("API server error: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := startSSHServer(ctx); err != nil {
			errChan <- fmt.Errorf("SSH server error: %w", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, stopping servers...")
		cancel()
	}()

	select {
	case err := <-errChan:
		cancel()
		return err
	case <-ctx.Done():
		log.Println("Waiting for servers to stop...")
		wg.Wait()
		log.Println("All servers stopped")
		return nil
	}
}

func runAPIServerCommand(cmd *cobra.Command, args []string) error {
	return startAPIServer(cmd.Context())
}

func runSSHServerCommand(cmd *cobra.Command, args []string) error {
	return startSSHServer(cmd.Context())
}

func startAPIServer(ctx context.Context) error {
	gnetcliClient, err := gnetcli.New(&cfg.Gnetcli)
	if err != nil {
		return fmt.Errorf("failed to create gnetcli client: %w", err)
	}
	defer gnetcliClient.Close()

	server, err := api.NewServer(cfg, annetService, routerInstance, gnetcliClient)
	if err != nil {
		return fmt.Errorf("failed to create API server: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.API.Bind, cfg.Server.API.Port)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: server.Router(),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("Starting API server on %s", addr)
	return httpServer.ListenAndServe()
}

func startSSHServer(ctx context.Context) error {
	sshServer, err := ssh.NewServer(cfg, annetService, routerInstance)
	if err != nil {
		return fmt.Errorf("failed to create SSH server: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.SSH.Bind, cfg.Server.SSH.Port)

	go func() {
		<-ctx.Done()
		sshServer.Stop()
	}()

	log.Printf("Starting SSH server on %s", addr)
	return sshServer.Start(addr)
}