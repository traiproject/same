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
	Viewport       viewport.Model
	AutoScroll     bool
	ActiveTaskName string
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

	case tea.WindowSizeMsg:
		// Split screen: 30% for task list, 70% for logs
		listWidth := int(float64(msg.Width) * 0.3)
		logWidth := msg.Width - listWidth - 4 // minus margins/borders

		m.Viewport.Width = logWidth
		m.Viewport.Height = msg.Height - 2 // minus header/footer space if any

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

			// Focus follows activity
			m.ActiveTaskName = msg.Name
			m.Viewport.SetContent(node.Logs.String())
			if m.AutoScroll {
				m.Viewport.GotoBottom()
			}
		}

	case telemetry.MsgTaskLog:
		if node, ok := m.SpanMap[msg.SpanID]; ok {
			node.Logs.Write(msg.Data)

			// Update viewport if we are looking at this task
			if node.Name == m.ActiveTaskName {
				// We append properly by setting content again.
				// Optimization: In a real app we might append line by line, but SetContent is safe.
				m.Viewport.SetContent(node.Logs.String())
				if m.AutoScroll {
					m.Viewport.GotoBottom()
				}
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


