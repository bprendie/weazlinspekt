package tui

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bprendie/weazlinspekt/internal/llm"
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
		b.WriteString(text)
	}
	return b.String()
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
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0D0D12")).
		Background(crushMint).
		Bold(true)
	if len(tok.Alternatives) < 2 {
		return style
	}
	top := tok.Alternatives[0].Probability
	gap := top - tok.Alternatives[1].Probability
	if gap < 0 {
		gap = -gap
	}
	if gap < inspektSplitThreshold {
		return style.Background(crushPink)
	}
	if top < 65 {
		return style.Background(crushGold)
	}
	return style
}

func (m model) togglePlayback() (model, tea.Cmd) {
	if !m.inspektMode || !m.playback.enabled {
		return m, nil
	}
	m.playback.playing = !m.playback.playing
	m.applyPlaybackCursor()
	m.renderMessages()
	if m.playback.playing {
		m.status = "inspektor playing 0.1x"
		return m, playbackTick()
	}
	m.status = playbackStatus("paused", m.playback)
	return m, nil
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
			return m, playbackTick()
		}
		m.status = playbackStatus("caught up", m.playback)
		return m, nil
	}
	m.applyPlaybackCursor()
	m.renderMessages()
	return m, playbackTick()
}

func playbackTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return playbackTickMsg{}
	})
}

func playbackStatus(prefix string, state playbackState) string {
	return prefix + " " + intString(state.cursor+1) + "/" + intString(len(state.tokens))
}

func intString(v int) string {
	return strconv.Itoa(v)
}
