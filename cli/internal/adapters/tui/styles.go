package tui

import (
	"github.com/charmbracelet/lipgloss"
	"go.trai.ch/same/internal/ui/style"
)

var (
	taskPendingStyle = lipgloss.NewStyle().
				Foreground(style.Slate)

	taskRunningStyle = lipgloss.NewStyle().
				Foreground(style.Iris).
				Bold(true)

	taskDoneStyle = lipgloss.NewStyle().
			Foreground(style.Green)

	taskErrorStyle = lipgloss.NewStyle().
			Foreground(style.Red)

	taskCachedStyle = lipgloss.NewStyle().
			Foreground(style.Slate).
			Faint(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(style.Iris).
			Bold(true)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			Background(style.Iris).
			Foreground(style.White)

	failureTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Padding(0, 1).
				Background(style.Red).
				Foreground(style.White)
)
