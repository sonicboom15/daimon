// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "daimon",
	Short: "The spirit that runs alongside your AI app",
	Long: `Daimon is a local sidecar runtime that gives your application a single,
stable HTTP interface to any LLM. Configure providers in a YAML file;
your app speaks plain HTTP and Server-Sent Events.`,
}

func init() {
	rootCmd.PersistentFlags().String("config", "examples/config.yaml", "path to YAML config file")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(runCmd)
}
