package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/klubi/orca/internal/agent"
	"github.com/klubi/orca/internal/apiserver"
	"github.com/klubi/orca/internal/config"
	"github.com/klubi/orca/internal/controller"
	"github.com/klubi/orca/internal/scheduler"
	"github.com/klubi/orca/internal/store"
	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

func newServeCmd() *cobra.Command {
	var (
		port    int
		host    string
		dataDir string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Orca control plane",
		Long:  "Start the Orca API server and all controllers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Build configuration with CLI overrides.
			cfg := config.DefaultConfig()
			if cmd.Flags().Changed("port") {
				cfg.Server.Port = port
			}
			if cmd.Flags().Changed("host") {
				cfg.Server.Host = host
			}
			if cmd.Flags().Changed("data-dir") {
				cfg.Store.DataDir = dataDir
			}

			// 2. Create logger.
			logger, err := zap.NewDevelopment()
			if err != nil {
				return fmt.Errorf("creating logger: %w", err)
			}
			defer logger.Sync()

			// 3. Ensure data directory exists and open BoltDB store.
			if err := os.MkdirAll(cfg.Store.DataDir, 0755); err != nil {
				return fmt.Errorf("creating data directory %s: %w", cfg.Store.DataDir, err)
			}

			boltStore, err := store.NewBoltStore(cfg.DBPath())
			if err != nil {
				return fmt.Errorf("opening store at %s: %w", cfg.DBPath(), err)
			}
			defer boltStore.Close()

			// 4. Create executor and runtime.
			executor := agent.NewExecutor(cfg.Agent.ClaudeCLI, logger)
			runtime := agent.NewRuntime(boltStore, executor, cfg, logger)

			// 5. Create scheduler.
			sched := scheduler.NewScheduler(boltStore, logger)

			// 6. Create controller manager and register controllers.
			mgr := controller.NewManager(boltStore, logger)

			agentPoolCtrl := controller.NewAgentPoolController(boltStore, runtime, logger)
			mgr.Register("AgentPoolController", agentPoolCtrl, []string{
				v1alpha1.KindAgentPool,
				v1alpha1.KindAgentPod,
			})

			devTaskCtrl := controller.NewDevTaskController(boltStore, sched, runtime, logger)
			mgr.Register("DevTaskController", devTaskCtrl, []string{
				v1alpha1.KindDevTask,
				v1alpha1.KindAgentPod,
			})

			healthCheckInterval := time.Duration(cfg.Agent.HealthCheckInterval) * time.Second
			healthCheckCtrl := controller.NewHealthCheckController(boltStore, runtime, healthCheckInterval, logger)
			mgr.Register("HealthCheckController", healthCheckCtrl, []string{
				v1alpha1.KindAgentPod,
			})

			// 7. Start controller manager.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := mgr.Start(ctx); err != nil {
				return fmt.Errorf("starting controller manager: %w", err)
			}

			// 8. Create and start API server.
			addr := cfg.ServerAddress()
			apiSrv := apiserver.NewServer(addr, boltStore, runtime, logger)

			// Print startup banner.
			banner := color.New(color.FgCyan, color.Bold)
			banner.Println("Orca Control Plane")
			fmt.Printf("   API Server: http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
			fmt.Printf("   Data Dir:   %s\n", cfg.Store.DataDir)
			fmt.Printf("   DB Path:    %s\n", cfg.DBPath())
			fmt.Println()

			// Start API server in a goroutine.
			errCh := make(chan error, 1)
			go func() {
				if err := apiSrv.Start(); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()

			// 9. Wait for interrupt signal for graceful shutdown.
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			select {
			case sig := <-sigCh:
				logger.Info("received shutdown signal", zap.String("signal", sig.String()))
			case err := <-errCh:
				logger.Error("API server error", zap.Error(err))
				cancel()
				mgr.Stop()
				return err
			}

			// Graceful shutdown with a 10-second deadline.
			fmt.Println()
			logger.Info("shutting down gracefully...")

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()

			// Stop controllers first.
			mgr.Stop()

			// Shutdown API server.
			if err := apiSrv.Shutdown(shutdownCtx); err != nil {
				logger.Error("API server shutdown error", zap.Error(err))
			}

			// Cancel the root context.
			cancel()

			logger.Info("Orca control plane stopped")
			return nil
		},
	}

	cmd.Flags().IntVar(&port, "port", 7117, "API server port")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "API server host")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (default: ~/.orca/data)")

	return cmd
}
