package interactive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelDefaults(t *testing.T) {
	m := NewModel("test-svc")
	if m.serviceName != "test-svc" {
		t.Errorf("serviceName = %q, want test-svc", m.serviceName)
	}
	if m.kind != "hertz" {
		t.Errorf("kind = %q, want hertz", m.kind)
	}
	if m.step != 0 {
		t.Errorf("step = %d, want 0", m.step)
	}
	if m.withDB {
		t.Error("withDB should default to false")
	}
	if m.input.Placeholder == "" {
		t.Error("placeholder should not be empty")
	}
}

func TestAutoModuleGeneratesReasonableSuggestion(t *testing.T) {
	cwd, _ := os.Getwd()
	base := filepath.Base(cwd)
	suggestion := autoModule("my-service")
	if suggestion == "" {
		t.Fatal("autoModule should not return empty")
	}
	if !strings.Contains(suggestion, "my-service") {
		t.Errorf("suggestion %q should contain service name", suggestion)
	}
	if !strings.Contains(suggestion, base) {
		t.Errorf("suggestion %q should contain cwd base %q", suggestion, base)
	}
}

func TestQuittingMarksFlag(t *testing.T) {
	m := NewModel("test-svc")
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2, ok := nm.(*model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", nm)
	}
	if !m2.quitting {
		t.Error("quitting should be true after esc")
	}
	if cmd == nil {
		t.Error("should return Quit command after esc")
	}
}

func TestCtrlCQuits(t *testing.T) {
	m := NewModel("test-svc")
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m2, ok := nm.(*model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", nm)
	}
	if !m2.quitting {
		t.Error("quitting should be true after ctrl+c")
	}
}

func TestEnterAdvancesSteps(t *testing.T) {
	tests := []struct {
		name         string
		initialStep  int
		inputValue   string
		wantStep     int
		wantKind     string
		wantWithDB   bool
		wantQuitting bool
	}{
		{
			name:        "step0 enter with value",
			initialStep: 0,
			inputValue:  "github.com/test/mymod",
			wantStep:    1,
			wantKind:    "hertz",
		},
		{
			name:        "step0 enter empty uses placeholder",
			initialStep: 0,
			inputValue:  "",
			wantStep:    1,
			wantKind:    "hertz",
		},
		{
			name:        "step1 enter h selects hertz",
			initialStep: 1,
			inputValue:  "h",
			wantStep:    2,
			wantKind:    "hertz",
		},
		{
			name:        "step1 enter k selects kitex",
			initialStep: 1,
			inputValue:  "k",
			wantStep:    2,
			wantKind:    "kitex",
		},
		{
			name:        "step1 enter K selects kitex",
			initialStep: 1,
			inputValue:  "K",
			wantStep:    2,
			wantKind:    "kitex",
		},
		{
			name:        "step1 enter other defaults to hertz",
			initialStep: 1,
			inputValue:  "x",
			wantStep:    2,
			wantKind:    "hertz",
		},
		{
			name:        "step2 enter n selects no database",
			initialStep: 2,
			inputValue:  "n",
			wantStep:    3,
			wantWithDB:  false,
		},
		{
			name:        "step2 enter p selects postgres",
			initialStep: 2,
			inputValue:  "p",
			wantStep:    3,
			wantWithDB:  true,
		},
		{
			name:        "step2 enter P selects postgres",
			initialStep: 2,
			inputValue:  "P",
			wantStep:    3,
			wantWithDB:  true,
		},
		{
			name:         "step3 enter y proceeds",
			initialStep:  3,
			inputValue:   "y",
			wantStep:     3,
			wantQuitting: false,
		},
		{
			name:         "step3 enter n cancels",
			initialStep:  3,
			inputValue:   "n",
			wantStep:     3,
			wantQuitting: true,
		},
		{
			name:         "step3 enter N cancels",
			initialStep:  3,
			inputValue:   "N",
			wantStep:     3,
			wantQuitting: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel("test-svc")
			m.step = tt.initialStep
			m.input.SetValue(tt.inputValue)

			nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m2, ok := nm.(*model)
			if !ok {
				t.Fatalf("Update returned unexpected type %T", nm)
			}
			if m2.step != tt.wantStep {
				t.Errorf("step = %d, want %d", m2.step, tt.wantStep)
			}
			if tt.wantKind != "" && m2.kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", m2.kind, tt.wantKind)
			}
			if m2.withDB != tt.wantWithDB {
				t.Errorf("withDB = %v, want %v", m2.withDB, tt.wantWithDB)
			}
			if m2.quitting != tt.wantQuitting {
				t.Errorf("quitting = %v, want %v", m2.quitting, tt.wantQuitting)
			}
		})
	}
}
