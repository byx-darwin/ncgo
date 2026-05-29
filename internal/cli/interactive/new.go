// Package interactive provides a TUI-based interactive prompt flow for
// ncgo new when required flags are missing and stdin is a terminal.
package interactive

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Result holds the values collected by the interactive flow.
type Result struct {
	Module string
	Kind   string
	WithDB bool
}

// model implements tea.Model for the interactive new flow.
type model struct {
	serviceName string
	module      string
	kind        string
	withDB      bool

	step     int // 0=module, 1=kind, 2=database, 3=confirm
	input    textinput.Model
	quitting bool
	err      error
}

// NewModel creates a new interactive flow model for the given service name.
// It auto-suggests a Go module path from the current directory.
func NewModel(serviceName string) *model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = autoModule(serviceName)
	ti.Width = 60
	ti.CharLimit = 200

	return &model{
		serviceName: serviceName,
		kind:        "hertz",
		step:        0,
		input:       ti,
	}
}

// autoModule returns a suggested Go module path based on the current
// directory and service name.
func autoModule(serviceName string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("github.com/%s/%s", filepath.Base(cwd), serviceName)
}

func (m *model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case 0: // module
		val := m.input.Value()
		if val == "" {
			val = m.input.Placeholder
		}
		m.module = val
		m.step = 1
		m.input.Reset()
		m.input.Placeholder = "h"
		m.input.Width = 1
		m.input.SetValue("h")
		return m, nil
	case 1: // kind
		val := m.input.Value()
		switch val {
		case "k", "K":
			m.kind = "kitex"
		default:
			m.kind = "hertz"
		}
		m.step = 2
		m.input.Reset()
		m.input.Placeholder = "n"
		m.input.Width = 1
		m.input.SetValue("n")
		return m, nil
	case 2: // database
		val := m.input.Value()
		m.withDB = val == "p" || val == "P"
		m.step = 3
		m.input.Reset()
		m.input.Placeholder = "y"
		m.input.Width = 1
		m.input.SetValue("y")
		return m, nil
	case 3: // confirm
		val := m.input.Value()
		if val == "n" || val == "N" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) View() string {
	switch m.step {
	case 0:
		return fmt.Sprintf("Service: %s\n\nModule path:\n%s\n\n(press enter to confirm, esc to cancel)\n",
			m.serviceName, m.input.View())
	case 1:
		return fmt.Sprintf("Module: %s\n\nKind:\n  [h] hertz (default)\n  [k] kitex\n\nChoice: %s",
			m.module, m.input.View())
	case 2:
		return fmt.Sprintf("Kind: %s\n\nDatabase:\n  [n] none (default)\n  [p] postgres\n\nChoice: %s",
			m.kind, m.input.View())
	default:
		dbLabel := "none"
		if m.withDB {
			dbLabel = "postgres"
		}
		return fmt.Sprintf("Confirm:\n  Service: %s\n  Module: %s\n  Kind: %s\n  Database: %s\n\n[y] proceed  [n] cancel\nChoice: %s",
			m.serviceName, m.module, m.kind, dbLabel, m.input.View())
	}
}

// Run starts the interactive flow and returns the collected options.
// If the user cancels, it returns nil, nil.
func Run(serviceName string) (*Result, error) {
	m := NewModel(serviceName)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm, ok := finalModel.(*model)
	if !ok {
		return nil, fmt.Errorf("interactive: unexpected final model type %T", finalModel)
	}
	if fm.quitting {
		return nil, nil // user cancelled
	}
	return &Result{
		Module: fm.module,
		Kind:   fm.kind,
		WithDB: fm.withDB,
	}, nil
}
