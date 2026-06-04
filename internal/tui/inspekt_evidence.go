package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bprendie/weazlinspekt/internal/llm"
)

func (m model) dieRollTag() string {
	if !m.dieRollActive() {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0D0D12")).
		Background(crushPink).
		Bold(true).
		Padding(0, 1).
		Render("DIE ROLL")
}

func (m model) dieRollState() string {
	if !m.inspektMode || !m.playback.enabled {
		return ""
	}
	label := "die off"
	style := lipgloss.NewStyle().Foreground(muted)
	if m.inspektDieRoll {
		label = "die on"
		style = lipgloss.NewStyle().Foreground(crushPink).Bold(true)
	}
	return style.Render(label)
}

func (m model) showDieRoll(width int) bool {
	return width >= 18 && m.dieRollActive()
}

func (m model) dieRollActive() bool {
	if !m.inspektMode || !m.inspektDieRoll {
		return false
	}
	_, ok := dieRollRank(m.inspektFrame)
	return ok
}

func dieRollRank(frame llm.LogprobFrame) (int, bool) {
	if frame.Token == "" || len(frame.Alternatives) == 0 {
		return 0, false
	}
	if frame.Alternatives[0].Token == frame.Token {
		return 1, false
	}
	for i, alt := range firstN(frame.Alternatives, 6) {
		if alt.Token == frame.Token {
			return i + 1, true
		}
	}
	return 0, true
}

func (m model) dieRollView() []string {
	rank, _ := dieRollRank(m.inspektFrame)
	face := dieFace(rank)
	lines := []string{m.styles.statusWarn.Render("sampled non-argmax")}
	for _, line := range face {
		lines = append(lines, lipgloss.NewStyle().Foreground(crushPink).Render(line))
	}
	return lines
}

func dieFace(rank int) []string {
	switch rank {
	case 2:
		return []string{"┌─────┐", "│ •   │", "│     │", "│   • │", "└─────┘"}
	case 3:
		return []string{"┌─────┐", "│ •   │", "│  •  │", "│   • │", "└─────┘"}
	case 4:
		return []string{"┌─────┐", "│ • • │", "│     │", "│ • • │", "└─────┘"}
	case 5:
		return []string{"┌─────┐", "│ • • │", "│  •  │", "│ • • │", "└─────┘"}
	case 6:
		return []string{"┌─────┐", "│ • • │", "│ • • │", "│ • • │", "└─────┘"}
	default:
		return []string{"┌─────┐", "│ ? ? │", "│  ?  │", "│ ? ? │", "└─────┘"}
	}
}

func (m model) inspektBar(width int, alt llm.TokenProbability, rank int) string {
	markerWidth := 2
	labelWidth := min(14, max(6, width/3))
	barWidth := max(4, width-labelWidth-markerWidth-10)
	fill := int((alt.Probability / 100) * float64(barWidth))
	fill = min(barWidth, max(0, fill))
	bar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Background(crushMint).Render(strings.Repeat(" ", fill)),
		lipgloss.NewStyle().Background(border).Render(strings.Repeat(" ", barWidth-fill)),
	)
	marker := m.inspektBarMarker(alt, rank)
	label := lipgloss.NewStyle().Width(labelWidth).Foreground(ink).Render(trimToWidth(visibleToken(alt.Token), labelWidth))
	return fmt.Sprintf("%s%s %s %5.1f%%", marker, label, bar, alt.Probability)
}

func (m model) inspektBarMarker(alt llm.TokenProbability, rank int) string {
	top := rank == 0
	selected := alt.Token == m.inspektFrame.Token
	switch {
	case top && selected:
		return lipgloss.NewStyle().Foreground(crushMint).Bold(true).Render("▶★")
	case selected:
		return lipgloss.NewStyle().Foreground(crushPink).Bold(true).Render("▶ ")
	case top:
		return lipgloss.NewStyle().Foreground(crushGold).Bold(true).Render("★ ")
	default:
		return "  "
	}
}
