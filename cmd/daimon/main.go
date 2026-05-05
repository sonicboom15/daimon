// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Command daimon is the daimon sidecar CLI.
package main

import (
	"log/slog"
	"os"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
