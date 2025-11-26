// Package main is the entry point for the bob build tool.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.trai.ch/bob/cmd/bob/commands"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/adapters/logger"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/app"
	"go.trai.ch/bob/internal/engine/scheduler"
)

func main() {
	os.Exit(run())
}

func run() int {
	// 0. Context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1. Parse config flag from command line before initializing everything
	configPath := parseConfigFlag()

	// 2. Infrastructure
	log := logger.New()
	configLoader := &config.FileConfigLoader{Filename: configPath}
	executor := shell.NewExecutor(log)

	// 3. Engine
	sched := scheduler.NewScheduler(executor)

	// 4. Application
	application := app.New(configLoader, sched)

	// 5. Interface
	cli := commands.New(application)

	// 6. Execution
	if err := cli.Execute(ctx); err != nil {
		log.Error(err)
		return 1
	}
	return 0
}

// parseConfigFlag extracts the --config/-c flag value from os.Args before cobra parses them.
// This allows us to initialize the config loader before creating the CLI.
func parseConfigFlag() string {
	configPath := "bob.yaml" // default value

	for i, arg := range os.Args {
		if arg == "--config" || arg == "-c" {
			if i+1 < len(os.Args) {
				configPath = os.Args[i+1]
			}
			break
		}
		// Handle --config=value format
		if len(arg) > 9 && arg[:9] == "--config=" {
			configPath = arg[9:]
			break
		}
	}

	return configPath
}
