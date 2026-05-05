// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	// Import components for side-effect registration.
	_ "github.com/sonicboom15/daimon/internal/components/anthropic"
	_ "github.com/sonicboom15/daimon/internal/components/llamacpp"
	_ "github.com/sonicboom15/daimon/internal/components/openai"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the daimon sidecar HTTP server",
	RunE:  runServe,
}

func runServe(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	srv, shutdown, err := buildSidecar(configPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(shutdownCtx)
	}()

	slog.Info("daimon listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
