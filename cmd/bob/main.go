// Package main is the entry point for the bob build tool.
package main

import (
	"fmt"
	"os"

	"go.trai.ch/bob/cmd/bob/commands"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/app"
	"go.trai.ch/bob/internal/engine/scheduler"
)

type StdLogger struct{}

func (l *StdLogger) Info(msg string) {
	fmt.Println(msg)
}

func (l *StdLogger) Error(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

func main() {
	// 1. Infrastructure
	logger := &StdLogger{}
	configLoader := &config.FileConfigLoader{Filename: "bob.yaml"}
	executor := shell.NewExecutor(logger)

	// 2. Engine
	sched := scheduler.NewScheduler(executor)

	// 3. Application
	application := app.New(configLoader, sched)

	// 4. Interface
	cli := commands.New(application)

	// 5. Execution
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
