// Package main is the entry point for the bob build tool.
package main

import (
	"fmt"
	"os"

	"go.trai.ch/bob/cmd/bob/commands"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/adapters/logger"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/app"
	"go.trai.ch/bob/internal/engine/scheduler"
)

func main() {
	// 1. Infrastructure
	log := logger.New()
	configLoader := &config.FileConfigLoader{Filename: "bob.yaml"}
	executor := shell.NewExecutor(log)

	// 2. Engine
	sched := scheduler.NewScheduler(executor)

	// 3. Application
	application := app.New(configLoader, sched)

	// 4. Interface
	cli := commands.New(application)

	// 5. Execution
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		os.Exit(1)
	}
}
