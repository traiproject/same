// Package main is the entry point for the bob build tool.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.trai.ch/bob/cmd/bob/commands"
	"go.trai.ch/bob/internal/adapters/cas"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/adapters/fs"
	"go.trai.ch/bob/internal/adapters/logger"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/app"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.trai.ch/zerr"
)

func main() {
	os.Exit(run())
}

func run() int {
	// 0. Context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1. Infrastructure
	log := logger.New()
	configLoader := config.NewFileConfigLoader("bob.yaml", log) // default value
	executor := shell.NewExecutor(log)
	walker := fs.NewWalker()
	resolver := fs.NewResolver()
	hasher := fs.NewHasher(walker)
	store, err := cas.NewStore(".bob/state")
	if err != nil {
		log.Error(zerr.Wrap(err, "failed to initialize build info store"))
		return 1
	}

	// 2. Engine
	sched := scheduler.NewScheduler(executor, store, hasher, resolver, log)

	// 3. Application
	application := app.New(configLoader, sched)

	// 4. Interface
	cli := commands.New(application)

	// 5. Set up config hook to update the config loader before command execution
	cli.SetConfigHook(func(configPath string) {
		configLoader.Filename = configPath
	})

	// 6. Execution
	if err := cli.Execute(ctx); err != nil {
		log.Error(err)
		return 1
	}
	return 0
}
