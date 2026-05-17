// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	// LLM providers.
	_ "github.com/sonicboom15/daimon/internal/components/llm/anthropic"
	_ "github.com/sonicboom15/daimon/internal/components/llm/gemini"
	_ "github.com/sonicboom15/daimon/internal/components/llm/llamacpp"
	_ "github.com/sonicboom15/daimon/internal/components/llm/mistral"
	_ "github.com/sonicboom15/daimon/internal/components/llm/openai"

	// Embedding components.
	_ "github.com/sonicboom15/daimon/internal/components/embedding/openai"

	// Session store components.
	_ "github.com/sonicboom15/daimon/internal/components/session/postgres"
	_ "github.com/sonicboom15/daimon/internal/components/session/redis"

	// Vector store components.
	_ "github.com/sonicboom15/daimon/internal/components/vector/chroma"
	_ "github.com/sonicboom15/daimon/internal/components/vector/inmemory"
	_ "github.com/sonicboom15/daimon/internal/components/vector/pgvector"
	_ "github.com/sonicboom15/daimon/internal/components/vector/qdrant"
	_ "github.com/sonicboom15/daimon/internal/components/vector/redis"

	// Graph store components.
	_ "github.com/sonicboom15/daimon/internal/components/graph/memgraph"
	_ "github.com/sonicboom15/daimon/internal/components/graph/neo4j"
)

var runCmd = &cobra.Command{
	Use:   "run [flags] -- <command> [args...]",
	Short: "Start the sidecar then launch and supervise a command",
	Long: `Starts the daimon sidecar server, waits until it is healthy, then
launches the given command as a child process.

When the child exits the sidecar shuts down. OS signals are forwarded to
the child so your app can handle them gracefully.

Example:
  daimon run --config config.yaml -- python app.py
  daimon run -- node server.js`,
	RunE: runRun,
}

func runRun(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command given — usage: daimon run [flags] -- <command> [args...]")
	}

	configPath, _ := cmd.Flags().GetString("config")

	srv, shutdown, err := buildSidecar(configPath)
	if err != nil {
		return err
	}

	// Start the sidecar server.
	srvErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()

	// Wait until the server is healthy before launching the child.
	slog.Info("waiting for sidecar", "addr", srv.Addr)
	if err := waitHealthy(srv.Addr, 5*time.Second); err != nil {
		_ = shutdown(context.Background())
		return err
	}
	slog.Info("sidecar ready", "addr", srv.Addr)

	// Start child process.
	child := exec.Command(args[0], args[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	if err := child.Start(); err != nil {
		_ = shutdown(context.Background())
		return fmt.Errorf("starting command: %w", err)
	}
	slog.Info("child started", "pid", child.Process.Pid, "cmd", args[0])

	// Forward OS signals to the child process.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			_ = child.Process.Signal(sig)
		}
	}()

	// Wait for child to exit or the sidecar to crash.
	childDone := make(chan error, 1)
	go func() { childDone <- child.Wait() }()

	var childErr error
	select {
	case childErr = <-childDone:
	case err := <-srvErr:
		slog.Error("sidecar crashed", "err", err)
		_ = child.Process.Kill()
		childErr = <-childDone
	}

	signal.Stop(sigCh)
	close(sigCh)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = shutdown(shutdownCtx)

	// Propagate the child's exit code.
	var exitErr *exec.ExitError
	if errors.As(childErr, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return childErr
}

// waitHealthy polls /healthz until the server responds 200 or the timeout expires.
func waitHealthy(addr string, timeout time.Duration) error {
	url := fmt.Sprintf("http://%s/healthz", addr)
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("sidecar at %s did not become healthy within %s", url, timeout)
}
