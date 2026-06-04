package tui

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bprendie/weazlinspekt/internal/llm"
	"github.com/bprendie/weazlinspekt/internal/storage"
)

type playbackRate int

const (
	playbackRealtime playbackRate = iota
	playbackHalf
	playbackTenth
	playbackStep
)

type playbackToken struct {
	Index        int
	Text         string
	ReceivedAt   time.Time
	Alternatives []llm.TokenProbability
}

type playbackState struct {
	enabled bool
	playing bool
	rate    playbackRate
	cursor  int
	tokens  []playbackToken
}

type playbackTickMsg struct{}

func newPlaybackState(enabled bool) playbackState {
	return playbackState{
		enabled: enabled,
		playing: enabled,
		rate:    playbackTenth,
		cursor:  -1,
	}
}

func (m *model) resetPlayback() {
	m.playback = newPlaybackState(m.inspektMode)
}

func (m *model) appendPlayback(chunk string, frames []llm.LogprobFrame) {
	m.absorbLogprobs(frames)
	if !m.playback.enabled {
		m.streamText += chunk
		m.reqOut = estimateTokens(m.streamText)
		return
	}
	if len(frames) == 0 {
		m.playback.tokens = append(m.playback.tokens, playbackToken{
			Index:      len(m.playback.tokens),
			Text:       chunk,
			ReceivedAt: time.Now(),
		})
	} else {
		for i, frame := range frames {
			text := frame.Token
			if i == 0 && chunk != "" {
				text = chunk
			}
			m.playback.tokens = append(m.playback.tokens, playbackToken{
				Index:        len(m.playback.tokens),
				Text:         text,
				ReceivedAt:   time.Now(),
				Alternatives: frame.Alternatives,
			})
		}
	}
	if m.playback.playing && m.playback.cursor < 0 {
		m.advancePlayback()
	}
	m.applyPlaybackCursor()
}

func (m *model) advancePlayback() bool {
	if m.playback.cursor+1 >= len(m.playback.tokens) {
		return false
	}
	m.playback.cursor++
	return true
}

func (m *model) rewindPlayback() bool {
	if m.playback.cursor < 0 {
		return false
	}
	m.playback.cursor--
	return true
}

func (m *model) applyPlaybackCursor() {
	m.streamText = m.playbackText(false)
	m.reqOut = estimateTokens(m.streamText)
	if m.playback.cursor >= 0 && m.playback.cursor < len(m.playback.tokens) {
		tok := m.playback.tokens[m.playback.cursor]
		m.inspektFrame = llm.LogprobFrame{Token: tok.Text, Alternatives: tok.Alternatives}
	}
}

func (m model) playbackText(highlight bool) string {
	if !m.playback.enabled {
		return m.streamText
	}
	var b strings.Builder
	for i := 0; i <= m.playback.cursor && i < len(m.playback.tokens); i++ {
		text := m.playback.tokens[i].Text
		if highlight && i == m.playback.cursor {
			b.WriteString(m.activeTokenStyle(m.playback.tokens[i]).Render(text))
			continue
		}
		if highlight {
			if style, ok := m.hesitationTokenStyle(m.playback.tokens[i]); ok {
				b.WriteString(style.Render(text))
				continue
			}
		}
		b.WriteString(text)
	}
	return b.String()
}

func (m model) playbackViewText() string {
	return wrapANSIText(m.playbackText(true), m.viewport.Width)
}

func (m model) playbackFullText() string {
	if !m.playback.enabled {
		return m.streamText
	}
	var b strings.Builder
	for _, tok := range m.playback.tokens {
		b.WriteString(tok.Text)
	}
	return b.String()
}

func (m model) activeTokenStyle(tok playbackToken) lipgloss.Style {
	style := baseTokenHighlight()
	if hesitation, ok := m.hesitationTokenStyle(tok); ok {
		return hesitation.Bold(true)
	}
	return style
}

func (m model) hesitationTokenStyle(tok playbackToken) (lipgloss.Style, bool) {
	if len(tok.Alternatives) < 2 {
		return lipgloss.Style{}, false
	}
	top := tok.Alternatives[0].Probability
	gap := top - tok.Alternatives[1].Probability
	if gap < 0 {
		gap = -gap
	}
	style := baseTokenHighlight()
	if gap < inspektSplitThreshold {
		return style.Background(crushPink), true
	}
	if top < 65 {
		return style.Background(crushGold), true
	}
	return lipgloss.Style{}, false
}

func baseTokenHighlight() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0D0D12")).
		Background(crushMint).
		Bold(true)
}

func (m model) togglePlayback() (model, tea.Cmd) {
	if !m.inspektMode || !m.playback.enabled {
		return m, nil
	}
	m.playback.rate = m.playback.rate.nextRate()
	m.playback.playing = true
	m.applyPlaybackCursor()
	m.renderMessages()
	if m.playback.rate == playbackStep {
		m.playback.playing = false
		m.status = "inspektor step " + playbackStatus("", m.playback)
		return m, nil
	}
	m.status = "inspektor " + m.playback.rate.label()
	return m, m.playbackTickCmd()
}

func (r playbackRate) nextRate() playbackRate {
	switch r {
	case playbackTenth:
		return playbackHalf
	case playbackHalf:
		return playbackRealtime
	case playbackRealtime:
		return playbackStep
	default:
		return playbackTenth
	}
}

func (r playbackRate) label() string {
	switch r {
	case playbackRealtime:
		return "1.0x"
	case playbackHalf:
		return "0.5x"
	case playbackTenth:
		return "0.1x"
	default:
		return "step"
	}
}

func (m model) playbackDelay() time.Duration {
	switch m.playback.rate {
	case playbackRealtime:
		return 20 * time.Millisecond
	case playbackHalf:
		return 50 * time.Millisecond
	case playbackTenth:
		return 100 * time.Millisecond
	default:
		return 0
	}
}

func (m model) playbackBadge() string {
	if !m.inspektMode || !m.playback.enabled {
		return ""
	}
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0D0D12")).
		Background(crushGold).
		Bold(true).
		Padding(0, 1)
	if m.playback.playing {
		return style.Render("play " + m.playback.rate.label())
	}
	return style.Render("pause " + m.playback.rate.label())
}

func (m model) renderPlaybackAssistant() bool {
	return m.inspektMode && m.playback.enabled && len(m.playback.tokens) > 0
}

func (m model) playbackMessageID() int64 {
	if len(m.messages) == 0 {
		return 0
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != "assistant" || strings.TrimSpace(last.Content) == "" {
		return 0
	}
	if last.Content != m.playbackFullText() {
		return 0
	}
	return last.ID
}

func (m model) playbackMessages() []storage.Message {
	msgID := m.playbackMessageID()
	if msgID == 0 {
		return m.messages
	}
	return m.messages[:len(m.messages)-1]
}

func (m model) renderPlaybackBlock() string {
	if !m.renderPlaybackAssistant() {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.styles.roleAI.Render("ai"))
	b.WriteString(" ")
	if badge := m.playbackBadge(); badge != "" {
		b.WriteString(badge)
	}
	b.WriteString("\n")
	b.WriteString(m.playbackText(true))
	b.WriteString("\n\n")
	return b.String()
}

func playbackTickFor(d time.Duration) tea.Cmd {
	if d <= 0 {
		return nil
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return playbackTickMsg{}
	})
}

func (m model) playbackTickCmd() tea.Cmd {
	if !m.playback.enabled || !m.playback.playing || m.playback.rate == playbackStep {
		return nil
	}
	if d := m.playbackDelay(); d > 0 {
		return playbackTickFor(d)
	}
	return nil
}

func (m model) playbackInitialCmd() tea.Cmd {
	if !m.playback.enabled || !m.playback.playing {
		return nil
	}
	return m.playbackTickCmd()
}

func (m model) playbackAdvanceCmd() tea.Cmd {
	if !m.playback.enabled || !m.playback.playing {
		return nil
	}
	return m.playbackTickCmd()
}

func (m model) stepPlayback(delta int) (model, tea.Cmd) {
	if !m.inspektMode || !m.playback.enabled {
		return m, nil
	}
	m.playback.playing = false
	if delta > 0 {
		m.advancePlayback()
	} else {
		m.rewindPlayback()
	}
	m.applyPlaybackCursor()
	m.status = playbackStatus("step", m.playback)
	m.renderMessages()
	return m, nil
}

func (m model) handlePlaybackTick() (tea.Model, tea.Cmd) {
	if !m.playback.enabled || !m.playback.playing {
		return m, nil
	}
	if !m.advancePlayback() {
		if m.thinking {
			return m, m.playbackTickCmd()
		}
		m.status = playbackStatus("caught up", m.playback)
		return m, nil
	}
	m.applyPlaybackCursor()
	m.renderMessages()
	return m, m.playbackTickCmd()
}

func playbackStatus(prefix string, state playbackState) string {
	return prefix + " " + intString(state.cursor+1) + "/" + intString(len(state.tokens))
}

func intString(v int) string {
	return strconv.Itoa(v)
}
