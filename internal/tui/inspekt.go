package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bprendie/weazlinspekt/internal/llm"
)

const inspektSplitThreshold = 10.0

type inspektSplit struct {
	Token        string                 `json:"token"`
	Gap          float64                `json:"gap"`
	Alternatives []llm.TokenProbability `json:"alternatives"`
}

func (m model) toggleInspektMode() (model, tea.Cmd) {
	m.inspektMode = !m.inspektMode
	m.resize()
	if m.inspektMode {
		m.status = "inspekt mode armed"
		m.renderMessages()
		return m, nil
	}
	m.status = "inspekt mode offline"
	m.renderMessages()
	return m, nil
}

func (m *model) absorbLogprobs(frames []llm.LogprobFrame) {
	if len(frames) == 0 {
		return
	}
	m.inspektFrame = frames[len(frames)-1]
	for _, frame := range frames {
		if split, ok := highEntropySplit(frame); ok {
			m.inspektSplits = append(m.inspektSplits, split)
		}
	}
}

func highEntropySplit(frame llm.LogprobFrame) (inspektSplit, bool) {
	if len(frame.Alternatives) < 2 {
		return inspektSplit{}, false
	}
	gap := frame.Alternatives[0].Probability - frame.Alternatives[1].Probability
	if gap < 0 {
		gap = -gap
	}
	if gap >= inspektSplitThreshold {
		return inspektSplit{}, false
	}
	return inspektSplit{Token: frame.Token, Gap: gap, Alternatives: frame.Alternatives}, true
}

func (m model) inspektView(width, height int) string {
	title := m.styles.roleTool.Render("inspektor")
	token := m.styles.statusLabel.Render("token") + " " + m.styles.statusValue.Render(visibleToken(m.inspektFrame.Token))
	if len(m.inspektFrame.Alternatives) == 0 {
		token = m.styles.help.Render("waiting for logprobs")
	}
	lines := []string{title, token, ""}
	for _, alt := range firstN(m.inspektFrame.Alternatives, 5) {
		lines = append(lines, m.inspektBar(width-4, alt))
	}
	if len(m.inspektSplits) > 0 {
		last := m.inspektSplits[len(m.inspektSplits)-1]
		lines = append(lines, "", m.styles.statusWarn.Render(fmt.Sprintf("hesitation gap %.2f%%", last.Gap)))
	}
	content := strings.Join(lines, "\n")
	return m.styles.sidebar.
		Width(max(20, width-2)).
		Height(max(5, height-2)).
		BorderForeground(crushPink).
		Render(content)
}

func (m model) inspektBar(width int, alt llm.TokenProbability) string {
	labelWidth := min(14, max(6, width/3))
	barWidth := max(4, width-labelWidth-9)
	fill := int((alt.Probability / 100) * float64(barWidth))
	fill = min(barWidth, max(0, fill))
	bar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Background(crushMint).Render(strings.Repeat(" ", fill)),
		lipgloss.NewStyle().Background(border).Render(strings.Repeat(" ", barWidth-fill)),
	)
	label := lipgloss.NewStyle().Width(labelWidth).Foreground(ink).Render(trimToWidth(visibleToken(alt.Token), labelWidth))
	return fmt.Sprintf("%s %s %5.1f%%", label, bar, alt.Probability)
}

func visibleToken(s string) string {
	if s == "" {
		return "<empty>"
	}
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, " ", ".")
	return s
}

func trimToWidth(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "~"
}

func firstN[T any](items []T, n int) []T {
	if len(items) <= n {
		return items
	}
	return items[:n]
}
