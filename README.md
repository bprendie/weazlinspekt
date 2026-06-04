# WeazlInspekt

![WeazlInspekt screenshot](weazlinspekt.png)

Rip the lid off the latent space. WeazlInspekt is a bare-metal, diagnostic overlay for local LLM execution. No cloud sludge, no web wrappers, no API telemetry pinging home. Just you, your terminal, and the raw probability vectors.

It's a distraction-free TUI for jacking into local models, armed with an Inspektor pane that exposes exactly what the machine is doing while it generates text. There is no magic. There are no ghosts. There are only probability distributions. WeazlInspekt splits your terminal to visualize that reality in real time.

## The Philosophy

Stop praying to the algorithm. When you grind away in a chat window for 18 hours, your brain hallucinates an entity on the other side. You start assuming the machine possesses a worldview or understands consequence. It doesn't.

WeazlInspekt nukes the conversational illusion. We intercept the `logprobs` from your local OpenAI-compatible vLLM node and map the exact token-by-token math directly to the screen. You aren't watching a wise entity make a decision; you are watching a statistical engine roll loaded dice.

## The Split View

Slam `ctrl+i` to split the TUI. Left pane: the Transcript. Right pane: the Inspektor.

With every SSE chunk that comes over the wire, the Inspektor updates to show the mathematical reality of the highlighted token. You get a stark, cyberpunk bar chart of the top alternatives, dynamically converting raw logprobs into percentages: `P = e^logprob * 100`. Pure signal, zero noise.

## Hesitation Markers And Playback

LLMs are fine-tuned to fake absolute confidence. Force them into a logical corner and watch the math fracture. WeazlInspekt tracks this entropy using hesitation markers:

- **Solid green:** The model is locked in. Usually structural syntax or obvious continuations.
- **Warning yellow & alert magenta:** The model is mathematically bleeding. The probabilities fracture into a 51/49 coin toss. The UI draws your eye to the exact millisecond a profound philosophical pivot was actually just a tight statistical guess.

The Inspektor pane renders a `top-k entropy` gauge and a historical entropy trace. Low entropy means the model is on rails. High entropy means it's violently guessing across competing continuations.

Toggle `ctrl+l` to flip the value column between percentages and raw `logprob` values. The API doesn't expose true pre-softmax logits, so we label this accurately as `logprob`. Cut the bullshit.

Want to run a forensic audit on a generation? Inspekt Mode buffers the token stream into RAM. Hit `'` to gear down the playback speed to `0.1x`, `0.5x`, `1.0x`, or full manual step mode. Jam `[` to step back, `]` to step forward. The transcript and Inspektor stay frame-synced.

## Immutable Vault Integration

We don't just show the math; we forge it into metal.

Backed by AES-GCM encrypted SQLite vaults, WeazlInspekt captures the highest-entropy logit splits during your session. Save a workspace (`ctrl+s`), and the vault retains the hesitation. Pull up a session from three months ago and audit exactly where the model's token predictions began to drift. Sovereign data, permanently logged.

## API Requirements

You need an endpoint that spits raw log probabilities.

- **vLLM:** Fully supported. We hit the `/v1/chat/completions` pipe, pass `logprobs: true` and `top_logprobs: 6`, and parse the nested payload.
- **Ollama:** Supported if your loaded model exposes token probability metrics.
- **`top_p`:** Lock `top_p: 1.0` on your server so the math doesn't get truncated before it hits your terminal.

## Defaults

Hardcoding endpoints is for amateurs. On first launch, WeazlInspekt drops a fresh `config.json` into the local app config directory:

- **Linux/macOS:** `~/.config/weazlinspekt/config.json`
- **Windows:** `%APPDATA%\weazlinspekt\config.json`

Read dynamically at runtime:

- `local-vllm`: `http://localhost:8000`
- `local-ollama`: `http://localhost:11434`
- `model`: `local-model`

## Run

```sh
go run ./cmd/weazlinspekt
```

## Install

```sh
./scripts/install.sh
```

On Windows, run the PowerShell installer:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1
```

The installer handles the yak hair. Linux/macOS builds get tucked into `~/.weazlinspekt/bin` and injected into your `PATH`. Windows installs to `%APPDATA%\weazlinspekt\bin`.

During setup, input your provider URL. Base URLs only. No `/v1` or `/api` suffixes. If you mess it up, the script quietly fixes it. Drop your tool API keys or type `-` to clear them.

Select your context window bounds: `small` (8k), `medium` (16k), `large` (32k), or `xl` (128k). Finally, lock it down with a local history password for the encrypted SQLite vault.

## Build From Source

WeazlInspekt is pure Go, but relies on `go-sqlite3`. You need Go 1.25+, CGO, and a C compiler. Grind it out.

### macOS

Install Go and Xcode tools:

```sh
xcode-select --install
```

Then build:

```sh
go build -o weazlinspekt ./cmd/weazlinspekt
go build -o weazlinspekt-setup ./cmd/weazlinspekt-setup
```

### Windows

Use MSYS2 to grab a C compiler:

```sh
pacman -S --needed mingw-w64-ucrt-x86_64-gcc
```

Build from PowerShell or the UCRT64 shell:

```sh
go build -o weazlinspekt.exe ./cmd/weazlinspekt
go build -o weazlinspekt-setup.exe ./cmd/weazlinspekt-setup
```

## Keys

- `enter`: blast message
- `up` / `down`: recall history
- `ctrl+m`: toggle mouse hijack vs terminal copy mode
- `ctrl+i` / `tab`: split the TUI / toggle Inspekt Mode
- `'`: cycle Inspekt playback speed
- `[` / `]`: step Inspekt playback back/forward
- `ctrl+g`: toggle Inspekt die-roll indicator
- `ctrl+l`: toggle Inspekt `%` vs `logprob`
- `ctrl+n`: spin up a new session
- `ctrl+r` / `ctrl+w`: open workspace saves
- `ctrl+s`: save active workspace
- `ctrl+t`: force context compaction (trim)
- `ctrl+u`: nuke the active session context
- `esc`: back to chat
- `ctrl+c`: kill process

## Inspekt Mode

Prompts are stateless by design. When you are in Inspekt Mode, the current prompt fires without the prior session context. Old conversation history does not poison the probability demonstration.

### Inspektor Evidence

We mark the evidence directly in the metal:

- `★`: The highest-probability token.
- `▶`: The token the sampler actually generated.
- `▶★`: Rank 1 match.

When `▶` and `★` split, the model sampled a non-argmax token. Hit `ctrl+g` to toggle the die-roll layer. WeazlInspekt tags the event with `[DIE ROLL]` and renders a static ASCII die face.

## Telemetry And Context Trimming

No blind execution. The status line feeds you live telemetry: token counts (`in`/`out`), velocity (`t/s`), and context pressure.

Running out of RAM? Jam `ctrl+t`. WeazlInspekt forces the active model to crush the session into a dense checkpoint summary. Small windows run close to the edge; large windows compact early so your 128k headroom isn't hijacked by a giant prompt tax.

Need a clean slate? Press `ctrl+u`. Nukes the active messages and tool history from SQLite, resets the context meter, and leaves you at a fresh prompt.

## TUI Feedback

While the GPU spins up, the TUI cycles demoscene status phrases: `hacking_the_gibson`, `jacking_into_the_matrix`, `wheezing_the_juice`.

Tool payloads stay hidden from the main transcript to keep the UI clean (`🔧 using tools`), but the raw bookkeeping is locked in the encrypted history. Workspace saves (`ctrl+s`) are fast snapshots, not a filing chore.

## Tool Support

WeazlInspekt supports function calling for local models that aren't braindead.

### Enabling Tools

Update your `config.json`:

```json
{
  "tools": {
    "enabled": true,
    "auto_execute_safe": true,
    "workspace_roots": ["/home/user/Code"]
  }
}
```

### Available Tools

- **General:** Calculator, Time, Weather, Markdown check, Stock lookup (Alpha Vantage), Web search (Brave Search), Fetch URL.
- **Local Workspace (Sandboxed):** `list_files`, `search_files`, `read_file`, `create_file` (no overwrites permitted).
- **Read-only Command:** Strict allowlist (`pwd`, `ls`, `git diff`, `go test`, etc.). No raw shell strings.
- **SQLite query:** Read-only `SELECT`, `WITH`, `EXPLAIN`, `PRAGMA`.
- **Local memory:** Encrypted storage (`remember`, `recall`, `forget`).

## Security

We encrypt the payload, but this isn't a hardware enclave. Your vault is only as tough as the password you forge.

- Safe tools are strictly read-only or create-only.
- File and shell tools are boxed into `workspace_roots`.
- Shell commands are allowlisted.
- URL fetching actively blocks private/local IP ranges.
- API keys live locally. Zero telemetry.

## License And Branding

Released under the MIT License. Use it, rip it, ship it.

The `WeazlInspekt` branding and aesthetic belong to this project. If you fork it into something else, change the name and the paint job.
