package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bprendie/weazlinspekt/internal/llm"
)

const (
	inspektSplitThreshold = 10.0
	weazlArtScale         = 2
	inspektGapWidth       = 2
	inspektPaneMinWidth   = 24
	inspektPaneOverhead   = 2
	chatMinWidth          = 20
)

type inspektSplit struct {
	Token        string                 `json:"token"`
	Gap          float64                `json:"gap"`
	Alternatives []llm.TokenProbability `json:"alternatives"`
}

type weazlArtLine struct {
	text      string
	row       int
	colOffset int
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
	innerWidth := max(20, width-4)
	innerHeight := max(5, height-4)
	title := m.styles.roleTool.Render("inspektor")
	if badge := m.playbackBadge(); badge != "" {
		title += " " + badge
	}
	if legend := m.inspektPlaybackLegend(); legend != "" {
		title += " " + legend
	}
	if tag := m.dieRollTag(); tag != "" {
		title += " " + tag
	}
	token := m.styles.statusLabel.Render("token") + " " + m.styles.statusValue.Render(visibleToken(m.inspektFrame.Token))
	if len(m.inspektFrame.Alternatives) == 0 {
		token = m.styles.help.Render("waiting for logprobs")
	}
	lines := []string{title, token, ""}
	if m.showDieRoll(innerWidth) {
		lines = append(lines, m.dieRollView()...)
		lines = append(lines, "")
	}
	for _, alt := range firstN(m.inspektFrame.Alternatives, 5) {
		lines = append(lines, m.inspektBar(width-4, alt))
	}
	if len(m.inspektSplits) > 0 {
		last := m.inspektSplits[len(m.inspektSplits)-1]
		lines = append(lines, "", m.styles.statusWarn.Render(fmt.Sprintf("hesitation gap %.2f%%", last.Gap)))
	}
	content := m.withInspektWeazl(lines, innerWidth, innerHeight)
	return m.styles.sidebar.
		Width(max(20, width-2)).
		Height(max(5, height-2)).
		BorderForeground(crushPink).
		Render(content)
}

func (m model) inspektPlaybackLegend() string {
	if !m.inspektMode || !m.playback.enabled {
		return ""
	}
	keyStyle := lipgloss.NewStyle().Foreground(crushPink).Bold(true)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		keyStyle.Render("[ ]"),
		m.styles.help.Render(" step "),
		keyStyle.Render("'"),
		m.styles.help.Render(" speed"),
	)
}

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

func inspektChatWidth(total int) int {
	w := max(chatMinWidth, (total*62)/100)
	reserved := inspektGapWidth + inspektPaneMinWidth + inspektPaneOverhead
	if total >= chatMinWidth+reserved && w+reserved > total {
		w = total - reserved
	}
	return w
}

func inspektPaneWidth(total, chatWidth int) int {
	return max(inspektPaneMinWidth, total-chatWidth-inspektGapWidth-inspektPaneOverhead)
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

func (m model) withInspektWeazl(lines []string, width, height int) string {
	available := height - len(lines) - 1
	if available < 6 || width < 18 {
		return strings.Join(lines, "\n")
	}
	art := clippedWeazl(width, available)
	if len(art) == 0 {
		return strings.Join(lines, "\n")
	}
	for len(lines)+len(art) < height {
		lines = append(lines, "")
	}
	for _, line := range art {
		rendered := colorWeazlLine(line.text, line.row, line.colOffset)
		lines = append(lines, lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(rendered))
	}
	return strings.Join(lines, "\n")
}

func clippedWeazl(width, height int) []weazlArtLine {
	source := scaledWeazlRows()
	if len(source) > height {
		source = source[len(source)-height:]
	}
	out := make([]weazlArtLine, 0, len(source))
	for _, line := range source {
		text := strings.TrimRight(line.text, " ")
		colOffset := 0
		if lipgloss.Width(text) > width {
			text, colOffset = rightRunes(text, width)
			colOffset *= weazlArtScale
		}
		out = append(out, weazlArtLine{text: text, row: line.row, colOffset: colOffset})
	}
	return out
}

func colorWeazlLine(line string, row, colOffset int) string {
	var b strings.Builder
	for col, r := range []rune(line) {
		sourceCol := colOffset + (col * weazlArtScale)
		b.WriteString(weazlCellStyle(row, sourceCol, r).Render(string(r)))
	}
	return b.String()
}

func scaledWeazlRows() []weazlArtLine {
	source := strings.Split(inspektWeazlArt, "\n")
	rows := make([]weazlArtLine, 0, (len(source)+weazlArtScale-1)/weazlArtScale)
	for row := 0; row < len(source); row += weazlArtScale {
		rows = append(rows, weazlArtLine{text: everyNthRune(source[row], weazlArtScale), row: row})
	}
	return rows
}

func everyNthRune(s string, n int) string {
	var b strings.Builder
	for i, r := range []rune(s) {
		if i%n == 0 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func fallbackWeazlRuneStyle(r rune) lipgloss.Style {
	switch r {
	case '█', '▌':
		return lipgloss.NewStyle().Foreground(lipgloss.Color("235"))
	case '▓':
		return lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	case '▒':
		return lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	case '░':
		return lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	case '╣', '╬', '║', '╫', '╠', '╙', '╚':
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	case '▀', '▄':
		return lipgloss.NewStyle().Foreground(lipgloss.Color("48"))
	case 'M':
		return lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	}
}

func rightRunes(s string, width int) (string, int) {
	r := []rune(s)
	removed := 0
	for len(r) > 0 && lipgloss.Width(string(r)) > width {
		r = r[1:]
		removed++
	}
	return string(r), removed
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

const inspektWeazlArt = `███████████████████████████████████████████████████████████████████████████████
████████████████████████████████▓╣╣╬████████████████████████████████████████████
███████████████████████▀▀▀▒░▄▒▒▒▒▒▒▒▒▒▒▒▒▒▓▀████████████████████████████████████
███████████████████▀▀░▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▓▒▒▀██████████████████████████████
█████████████████▀░▒▒▒▒▒▒▒╬▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒░▓▓▓▓▒▀███████████████████████████
███████████████▒░▒▒▒▒▀█▄░░▓▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▓║▓▓▓▓▓▒██████████████████████████
██████████████▀▒▒▒▒▒░▄█▓▒▓▓▓▓▓▓▓▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▓▓▓▓▓▓▒█████████████████████████
██████████████▒▒░██▓▓▓▓▒▒▓▓▒▒▒▓▓▓▓█▒▒▒▒▒▒▒▒▒▒▒▒▌▓▓▓▓▓▓▒▓████████████████████████
██████████████▒▒▒░▄▓▓▒▒▒▒▒▓▓▒▓▓▒▒▓▓█▒▒▒▒▒▒▒▒▒▒▒▓║▓▓▓▓▓▓▒████████████████████████
██████████████▒░█▓▓█▓█▒▓▓███▓▓█▓▓▓▓█▒▒▒▒▒▒▒▒▒▒▒▓║▓▓▓▓▓▒▒████████████████████████
██████████████▒▒▒▒╣██▓▀▒▒▒▒▒▄▒▒█▒╬▒▓█▒▒▒▒▒▒▒▒╣▒▒╬▓▒▓▓▀▒▒▒▒▓█████████████████████
████████▓▒▒▒▓█▒▒▒▒▓▌▀▒▒▒▄▒▄▄▄▄██▓▓▓██▓▓▓▓▓╣╣▒▒▒▓█▓▓▒▒▓██▓▓█▓████████████████████
███████▓▓▓▓▒▓█▒╬████▓▓▓▓▓▓▓▓▓▒▒▓▓▓▓▓▓▓▓▓▓▓▓▓█▓██▓▒▓▓█▓▓▓▒▓▓▓▒███████████████████
███████▌▓▓█▓▓▓▓▓▓▓▒▒▒▒▓▓▓▒▒▓▒▓▓▒▒▓▒▓▓▒▒▓▓▓▓▒▒▒▒▒▓▒▓█▓▓▓▓▒▒▒▓▒███████████████████
███████▓▓▓█▓▓▒▓▒▓▓▓▓▓▒▓▒▓▒▒▓▓▓▓▓▒▓▓╬▌▒▒╣▒▒▓▓███▓▀█▓▓▒███▓▒▒▓▓█▓▓▒▀▀█████████████
████████▓▓▓▓▓▓▀▒▒▒▒▒▒║▒▒▓▓▓▒▒▓▒▓▀░▄▀▀▀▀▀▒▒▓▄▄╣▓▓▓▓▓▓▓▓▓▓▓▒▓▓█▓▓▓▓▓▓▓▄▒▀▀████████
██████████▓█▒▒▓███▒▒▒▒░▓▒▓▓▓▓▒▓▓▒▓╠▓▓▓▓▀▀▀▓▓▓▓▌▒▓▓▒▒▓▓▓▓▓▓██▓▓▓▓▓▓▓▓▓▓▓▓▓▓██████
███████████▒▒▓▓▓███▓▒░░█▓▓▓▓▓▓▓▓▒█▒▓▓▒▒M╙╚▒║▓▓▓╣▓█▒▓▓▓▓▓▓▓▓█████████████████████
███████████▓▒▓▓██▀▀▓███▓▓▓▓▓▓▓▓▓▓█▒▓▓▒▒░░░╣║▓▓█▒▓█▒▒▓▓▓▓▓▓▓█████████████████████
███████████▓▒▒▓▓█▌▄▄██▓▒▓▒▒▒▓▒▒▒▓█▒▓▓▓▓▄▄▄▓▓▓▓▓▒▓▓█▓▓▓▓▓▓▀██████████████████████
███████████▒▓▒▓▓▓▓▓█▓▓▒▓▒▒▒▒║▒▒▓▒▓██▓▓▓▓▓▄▓▓▓▓▓█▓▓▓▒▒▓████▓█████████████████████
████████████▌▒▒▒▓▒▓▓▓▓▒░▒▒▒▒▒▒▒║█▓▓▓▓▓▓▓█▓▀▀▀▀▀▀▀▀▓█▓▒▓█▓▓▓█████████████████████
████████████╫▌╬▒▒▒▓▀░▀░▄▄▄▒▄░░░░░▒░░▀▓▓▓▓▀▀▀▒░╬▒╣▒▄▒▒████▓▓█████████████████████
███████████████▒▒█░▒▒▄▓▓▓▓▓▓▓▓█░▒▒▒▒▒░▒▒▓░╣╣▒╣▒▒▒▒▓▓██▓▓▓▓▓█████████████████████
██████████████████░░▒▀██▓▓▓▓███░▒▒▒░░▒▒▒▒▒▒▒▒▒▓▀▓▓▓▓▓▓▓▓█▓█▓▀███████████████████
██████████████████▌░▒▒░▀███▓▓▒░▒▒░▒▄░░▀█▓▒▒▓█▀▓▒▓▓▓▓▓▓▓▓▓▓▒██▓██████████████████
████████████████████▒▓▓▒▒▒███▓▒▒▒╣╣▒▒██▒▓█▓▓▓▒▒▒▒▓▓▓▓▓▒▓▓▓▓╫████████████████████
████████████████████████▒▒▒▒▒▒▒▒▒▒▓▒███▓▓▓▓▒▒▓▀▓▄▓▓▓▓▓▓▓▓▓▓█████████████████████
█████████████████████████▓██████████▓▓▓▓▓▌▓▒▓▓▒▒░░▓▒▓▓▒▓▓▓▓▓████████████████████
██████████████████████████▓▓▓███▓▓▓▓▓▓▓▒▒▒▓▒▒▒▒▒░▄▓▓▓▒▓▒▒▓▓▒████████████████████
██████████████████████████▓▓▒▓▒▒▓▒▓▒▒▒▒▒▒▒▒▒▒▒▒▒░▒▓▓▒▓▒▓▓▓▓▓╬███████████████████
██████████████████████████▓▌▓▒▒▀▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒░▀▓▒▒▒▓▓▓▓▓▓▓▀█████████████████
███████████████████████████▒▓▒▒▒░▒╣▒▓▒╣▒▒▒▒▒▒▒▒▒▒░▓▓▓▒▓▓▓▓▓▓▓▓▀█████████████████
█████████████████████████▀▒▓▒▒▒▒░▒▒░▒▒▒▒▒▒▒▒▒▒▒▒▒▒▓▓▒▓▓▒▓▓▓▓▓▓▓▓▓███████████████
█████████████████████████▒▓▓▓▒▒▒▒▒▒▒▒▒▒▒▒▒▒▒░▒▒▒▒░▓▓▒▓▓▓▓▓▓▓▓▓▓▓▓███████████████
██████████████████████▓▓▒▓▒▓╬░▒▒▒▒░▒▒▒▒▒▒▒▒▒║▒░▒▒░╩▓▓▓▓▒▓▓▓▒▒▒▓▓▓▓▓█████████████`
