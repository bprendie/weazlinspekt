# Inspektor Playback Workbook

Date: 2026-06-04

Status: discussion workbook, intentionally uncommitted until the design settles.

## Goal

Turn Inspektor from a live-only probability display into a forensic playback surface. The user should be able to pause, step, rewind, and replay an assistant response while keeping the visible transcript and token probability chart locked to the exact same generation point.

## Proposed Controls

- `'`: play/pause
- `[`: step back one token
- `]`: step forward one token
- Playback rates:
  - realtime
  - 0.5x
  - 0.1x
  - step/manual

Open detail: we need a key for cycling speed, unless speed is selected through an Inspektor pane control. Candidate keys could be `;`/`.` or a small mode cycle on `'` when paused, but that needs care to avoid surprising chat input behavior.

Decision: Inspektor mode starts at `0.1x` playback by default. Fast local models can emit far more tokens than a human can inspect, so the diagnostic mode should privilege legibility over raw streaming speed.

Decision: playback controls are active only when Inspektor mode is active. Outside Inspektor mode, `'`, `[`, and `]` remain normal chat input.

## Behavioral Model

Today, WeazlInspekt streams provider chunks directly into `streamText` and updates the current logprob frame as events arrive. The proposed model separates capture from presentation:

1. Capture receives the live SSE stream as fast as vLLM emits it.
2. Capture appends token events into an in-memory playback buffer.
3. Presentation reads from that buffer according to playback controls.
4. Transcript text and Inspektor logits render from the same playback cursor.
5. On completion, the full playback session is saved to the vault.

This means the model can keep generating while the UI is paused. The app is pausing playback, not vLLM inference.

## Core Invariant

There must be one authoritative playback cursor.

At cursor `N`:

- the assistant transcript contains tokens `0..N`
- the Inspektor pane shows token `N` and its top alternatives
- stepping backward decrements the same cursor and updates both panes
- stepping forward increments the same cursor and updates both panes

No view should independently accumulate text once playback mode exists.

Decision: if the user steps back 15 tokens, the chat pane rolls back 15 tokens too. The Inspektor pane shows the probability distribution for the active cursor position so the user can inspect what the model was about to emit next.

## Presentation Argument

The talk goal is not just to show that logits exist. It is to make the audience feel the gap between "wisdom" language and the mechanical probability choices underneath the text.

### Active Token Highlight

Decision: playback should visibly highlight the active token in the left pane.

At cursor `N`, the assistant transcript should render tokens before `N` normally, token `N` with a distinct highlighted style, and tokens after `N` hidden unless the user steps or plays forward.

Impact:

- The audience can connect the exact visible word/token to the Inspektor distribution.
- Step mode becomes a left-to-right model autopsy instead of a normal transcript plus a detached chart.
- When playback pauses on a high-entropy point, the highlighted token tells the audience where to look in the sentence.

### Entropy And Margin Warnings

Decision: Inspektor should visually distinguish low-certainty token choices.

Useful metrics:

- `confidence`: top token probability
- `gap`: top1 probability minus top2 probability
- optional `entropy`: Shannon entropy across the visible top alternatives

Recommended first trigger:

- Normal: top1 is confident and gap is wide
- Warning: top1 probability below roughly `65%`
- Coin flip: top1/top2 gap below roughly `10%`

The narrow-margin trigger is more rhetorically useful than top1 confidence alone. A `52/48` split between two semantic alternatives is a clearer rebuttal to "machine wisdom" than a syntactically constrained `99.9%` punctuation token.

Visual language:

- Normal token: green/mint
- Low confidence: gold/yellow
- Near tie: pink/red
- Inspektor pane should show a short label such as `gap 3.2%` or `coinflip`

### Hesitation Markers In Transcript

Decision: the transcript should reflect hesitation, not only the Inspektor chart.

Candidate behavior:

- Current active token gets a cursor/highlight style.
- If the active token has low confidence, the highlight shifts to warning gold.
- If the active token is a near tie, the highlight shifts to alert pink/red.
- Previously generated high-entropy tokens may retain a subtle marker or underline so the transcript shows where the model hesitated.

Impact:

- The left pane itself becomes evidence.
- The audience can scan the sentence and see where probability, not understanding, drove meaningful turns.

### Parser Confidence

The vLLM chat-completions logprob schema is already the target parser shape:

```json
{
  "choices": [
    {
      "delta": {"content": ".town"},
      "logprobs": {
        "content": [
          {
            "token": ".town",
            "logprob": -0.0001,
            "top_logprobs": [
              {"token": ".town", "logprob": -0.0001},
              {"token": ".village", "logprob": -5.321}
            ]
          }
        ]
      }
    }
  ]
}
```

Decision: add a fixture test for this exact schema during implementation. The parser already handles this shape today, but a dedicated fixture gives us confidence that future playback changes do not break vLLM logprob parsing.

## Data Shape

Candidate in-memory event:

```go
type PlaybackToken struct {
    Index        int
    Text         string
    ReceivedAt   time.Time
    Alternatives []llm.TokenProbability
}
```

Candidate playback session:

```go
type PlaybackSession struct {
    MessageID    int64
    SessionID    string
    Provider     string
    Model        string
    StartedAt    time.Time
    CompletedAt  time.Time
    Tokens       []PlaybackToken
}
```

Important nuance: SSE chunks are not always one token of text in every provider/server behavior. vLLM logprobs usually arrive per generated token in `logprobs.content[]`, but `delta.content` may not always map one-to-one. We should normalize provider chunks into token frames where each frame owns:

- display text appended for that token
- logprob alternatives for that token
- arrival timestamp

## Vault Persistence

Current vault storage saves assistant text plus high-entropy splits in `messages.logit_splits`.

For forensic replay, `logit_splits` is not enough. We need the full token timeline.

Candidate schema:

```sql
create table if not exists playback_sessions (
    id integer primary key autoincrement,
    message_id integer not null references messages(id) on delete cascade,
    session_id text not null references sessions(id) on delete cascade,
    provider text not null,
    model text not null,
    tokens text not null,
    started_at datetime not null,
    completed_at datetime
);
```

`tokens` can be JSON initially. It is simple, encrypted-at-rest by the vault file boundary, and low ceremony. If queryability becomes important, we can split to `playback_tokens`, but that is probably premature.

Decision: full token replay data should be encrypted like message content. Playback tokens expose the assistant response and probability trail, so plaintext JSON metadata is not acceptable for forensic sessions.

## UI State

Candidate fields on `model`:

```go
playback playbackState
```

Candidate state:

```go
type playbackState struct {
    enabled bool
    live bool
    playing bool
    rate playbackRate
    cursor int
    tokens []PlaybackToken
    startedAt time.Time
    completedAt time.Time
}
```

Rates:

```go
type playbackRate int

const (
    playbackRealtime playbackRate = iota
    playbackHalf
    playbackTenth
    playbackStep
)
```

## Tea Message Flow

Keep thread safety by preserving the existing channel-to-`tea.Msg` boundary.

Possible messages:

```go
type playbackTokenMsg struct {
    token PlaybackToken
}

type playbackTickMsg struct{}
```

Capture path:

- provider stream goroutine parses SSE
- stream goroutine sends `streamEvent`
- `handleStreamEvent` appends token frames to `playback.tokens`
- if playback is live/playing, it advances cursor through tea messages

Presentation path:

- cursor changes rebuild `streamText` from `tokens[:cursor+1]`
- Inspektor frame comes from `tokens[cursor].Alternatives`
- `renderMessages()` becomes cursor-aware during assistant generation/replay

## Playback Timing

Realtime can use recorded inter-token deltas:

- token 0 appears immediately
- token N delay = `tokens[N].ReceivedAt - tokens[N-1].ReceivedAt`

0.5x and 0.1x need naming clarity:

- Usually `0.5x` means half speed, so delays are doubled.
- `0.1x` means one-tenth speed, so delays are multiplied by 10.

For usability, clamp long delays. Local model stalls could otherwise make playback feel broken.

Candidate clamp:

- min delay: 20ms
- max delay: 1500ms realtime
- scaled delay respects max maybe 5000ms

## Live Streaming Semantics

Question to settle: when Inspektor mode is active during live generation, should the user see:

1. Live playback by default, with cursor following capture.
2. Paused at token 0 until the user presses play.
3. Live unless they press pause, then capture continues in background.

Decision: option 3. It preserves current chat behavior and makes pause intuitive. In Inspektor mode, playback starts at `0.1x` by default rather than live realtime.

When paused:

- vLLM still streams in background
- buffer grows
- transcript/Inspektor stay at cursor
- status should show capture progress separately from playback cursor, e.g. `paused 18/73`

## Step Back During Live Capture

If the user steps back while capture is still running:

- playback enters paused/manual mode
- capture continues appending tokens
- `]` can step forward until it reaches latest captured token
- play resumes from current cursor and may eventually catch up to live

Once caught up, the cursor can either:

- keep following live capture, or
- continue playback timing from new arrivals

Recommendation: follow live once caught up unless the rate is explicitly slower than realtime.

## Replaying Saved Sessions

We need a later interaction for selecting saved playback sessions. Possible approaches:

- Phase 1 can replay the most recent assistant response only.
- Add playback data to existing message/session replay.
- When an assistant message has playback data, Inspektor can enter replay mode for that message.
- A future `ctrl+p` or message selection flow could pick assistant messages with recorded playback.

Decision: a message picker can wait. Phase 1 may target the latest assistant response.

Open question: the current TUI does not have message focus/selection inside chat. We may need a small picker listing assistant responses with playback data in a later phase.

## Risks

- Token/frame alignment: `delta.content` and `logprobs.content` must be normalized carefully.
- Memory pressure: long responses with top-5 alternatives are not huge, but full playback for many sessions can grow if kept in RAM.
- Vault privacy: playback JSON may expose full assistant text via token text if stored unencrypted.
- Input collisions: `[`, `]`, and `'` are normal text input keys. They should only control playback when Inspektor mode is active and probably when the input is not focused, or we need clear mode semantics.

## Suggested Implementation Sequence

1. Define `PlaybackToken` and normalize vLLM SSE chunks into token frames.
2. Add in-memory playback state and cursor-driven rendering.
3. Add play/pause and step controls in Inspektor mode.
4. Add playback tick scheduling for realtime, 0.5x, and 0.1x.
5. Persist completed playback sessions to the vault.
6. Add replay loading for saved assistant messages.

## Decisions Needed

- What key cycles playback speed?
- Should `[`, `]`, and `'` be intercepted only in Inspektor mode?
- Should playback JSON be encrypted like message content?
- Should a live paused stream hide future text from the transcript until playback catches up?
- How should users select an older assistant message for replay?

## Phase Plan Draft

### Phase 1: In-Memory Cursor Playback

Objective: make live Inspektor mode cursor-driven without database changes.

Scope:

- Add normalized token frame buffering in RAM.
- Add one authoritative playback cursor.
- Render assistant stream text from buffered tokens up to the cursor.
- Render Inspektor probabilities from the same cursor.
- Enable Inspektor-only controls:
  - `'`: play/pause
  - `[`: step back one token
  - `]`: step forward one token
- Default Inspektor playback rate to `0.1x`.
- Preserve normal chat input behavior outside Inspektor mode.

Exit criteria:

- During generation, stepping back visibly rolls back both transcript text and Inspektor token probabilities.
- Capture can continue while playback is paused.
- Playing resumes from the current cursor.

### Phase 2: Playback Timing Modes

Objective: make playback rates explicit and reliable.

Scope:

- Record token arrival timestamps.
- Add scheduler ticks for:
  - realtime
  - 0.5x
  - 0.1x
  - manual step
- Add a speed-selection key or pane control.
- Clamp pathological delays so playback remains usable.

Exit criteria:

- `0.1x` default is readable on fast models.
- Realtime approximates recorded token cadence.
- Pausing and stepping remain deterministic.

### Phase 3: Vault Persistence

Objective: save full encrypted token playback sessions.

Scope:

- Add storage schema for playback sessions.
- Encrypt token replay JSON with the same vault key path used for message content.
- Persist completed playback data linked to the assistant message.
- Keep existing `messages.logit_splits` as a lightweight query index.

Exit criteria:

- A completed assistant response has full encrypted replay data in the vault.
- Existing sessions migrate cleanly.
- Tests cover save/load and encryption round trip.

### Phase 4: Latest Response Replay

Objective: replay the most recent assistant response from the vault.

Scope:

- Load latest assistant playback session for the active chat.
- Re-enter Inspektor replay mode using saved token frames.
- Support the same controls: play/pause, step back, step forward, speed.

Exit criteria:

- Restarting the app and reopening the session can replay the latest assistant response forensically.

### Phase 5: Historical Replay Picker

Objective: select older assistant responses with playback data.

Scope:

- Add a picker/list for assistant messages that have playback sessions.
- Display timestamp, model, and short response preview.
- Load selected playback into Inspektor replay mode.

Exit criteria:

- Any recorded assistant response in the session can be selected and replayed.
