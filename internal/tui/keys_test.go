package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bprendie/weazlinspekt/internal/config"
)

func TestTabTogglesInspektMode(t *testing.T) {
	m := New(config.Default(), "", nil, nil).(model)
	m.mode = modeChat

	updated, _, handled := m.handleGlobalKey(tea.KeyMsg{Type: tea.KeyTab})
	if !handled {
		t.Fatal("tab key was not handled")
	}
	next := updated.(model)
	if !next.inspektMode {
		t.Fatal("inspekt mode was not enabled")
	}

	updated, _, handled = next.handleGlobalKey(tea.KeyMsg{Type: tea.KeyTab})
	if !handled {
		t.Fatal("second tab key was not handled")
	}
	next = updated.(model)
	if next.inspektMode {
		t.Fatal("inspekt mode was not disabled")
	}
}
