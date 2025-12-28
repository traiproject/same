package tui

import (
	"bytes"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"go.trai.ch/bob/internal/adapters/telemetry"
)

type TaskStatus string

const (
	StatusPending TaskStatus = "Pending"
	StatusRunning TaskStatus = "Running"
	StatusDone    TaskStatus = "Done"
	StatusError   TaskStatus = "Error"
)

type TaskNode struct {
	Name   string
	Status TaskStatus
	Logs   bytes.Buffer
	Cached bool
}

type Model struct {
	Tasks      []TaskNode
	TaskMap    map[string]*TaskNode
	SpanMap    map[string]*TaskNode
	Viewport   viewport.Model
	AutoScroll bool
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case telemetry.MsgInitTasks:
		m.Tasks = make([]TaskNode, len(msg.Tasks))
		m.TaskMap = make(map[string]*TaskNode, len(msg.Tasks))
		m.SpanMap = make(map[string]*TaskNode)
		for i, name := range msg.Tasks {
			m.Tasks[i] = TaskNode{
				Name:   name,
				Status: StatusPending,
			}
			m.TaskMap[name] = &m.Tasks[i]
		}

	case telemetry.MsgTaskStart:
		if node, ok := m.TaskMap[msg.Name]; ok {
			node.Status = StatusRunning
			m.SpanMap[msg.SpanID] = node
		}

	case telemetry.MsgTaskLog:
		if node, ok := m.SpanMap[msg.SpanID]; ok {
			node.Logs.Write(msg.Data)
			if m.AutoScroll {
				m.Viewport.GotoBottom()
			}
		}

	case telemetry.MsgTaskComplete:
		if node, ok := m.SpanMap[msg.SpanID]; ok {
			if msg.Err != nil {
				node.Status = StatusError
			} else {
				node.Status = StatusDone
			}
		}
	}

	return m, cmd
}

func (m Model) View() string {
	return ""
}
