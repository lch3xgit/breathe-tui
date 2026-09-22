# breathe-tui

`breathe-tui` is a small, offline terminal breath pacer written in Go. The current release line is v0.1.x; command behavior and presentation may still evolve before v1.0.0.

## Features

- Six built-in practices with deterministic, deadline-based timing
- Compact alternate-screen interface with a persistent wordmark when space permits
- Bracketed shaded breath meter, responsive sizing, and clean shell restoration
- Space to pause or resume, with paused time excluded from the session
- Optional terminal bell at live phase changes
- Plain line-oriented output when interactive terminal mode is unavailable

## Practices

Patterns are listed as inhale : hold full : exhale : hold empty, in seconds. A zero means that phase is omitted.

| Command | Practice | Pattern | Default |
| --- | --- | --- | --- |
| `coherence` | Coherence | 5.5:0:5.5:0 | ~5 minutes |
| `calm` | Calm | 4:0:6:0 | 5 minutes |
| `upshift` | Upshift | 6:0:4:0 | 5 minutes |
| `box` | Box Breath | 4:4:4:4 | ~5 minutes |
| `circular` | Circular Flow | 2:0:2:0 | 5 minutes |
| `478` | 4-7-8 | 4:7:8:0 | 8 breaths |

Duration-based sessions treat five minutes as a target and finish the current breath cycle instead of stopping mid-cycle. As a result, Coherence completes at 5:08 and Box Breath at 5:04; Calm, Upshift, and Circular Flow complete at 5:00. The 4-7-8 practice always completes eight full breaths.

## Usage

```text
breathe
breathe coherence
breathe calm
breathe upshift
breathe box
breathe circular
breathe 478
breathe --sound box
breathe help
breathe version
```

Running `breathe` without a practice starts Coherence.

Controls in an interactive session:

- Space pauses or resumes.
- `q` or `Q` ends the session.
- Ctrl+C ends the session.

`--sound` sends one terminal-bell character at each live phase transition. Whether that produces an audible sound depends on terminal and operating-system settings.

## Install from GitHub Releases

Release archives and `SHA256SUMS` are published on the [GitHub Releases page](https://github.com/lch3xgit/breathe-tui/releases). The v0.1.0 binaries are not code-signed, so the operating system may display a warning. You can inspect the source and verify the archive against the published SHA-256 checksum.

### Windows

Download `breathe_0.1.0_windows_amd64.zip`, extract it, and place `breathe.exe` in a user-owned directory included in `PATH`. Then verify the installation:

```powershell
breathe version
```

### macOS and Linux

Download the `.tar.gz` archive matching the operating system and architecture, extract it, and move `breathe` to a user-owned directory in `PATH`. For example, after substituting the downloaded archive name and directory:

```sh
mkdir -p ~/.local/bin
tar -xzf breathe_0.1.0_linux_amd64.tar.gz
install -m 0755 breathe_0.1.0_linux_amd64/breathe ~/.local/bin/breathe
breathe version
```

Your shell must include `~/.local/bin` in `PATH`. If needed, ensure the extracted binary is executable with `chmod +x breathe`.

## Build from source

Go 1.23 or newer is required.

```sh
git clone https://github.com/lch3xgit/breathe-tui.git
cd breathe-tui
go test ./...
go build -o breathe .
```

On Windows, use `go build -o breathe.exe .`. Move the resulting executable to a directory in `PATH` if desired. Ordinary source builds intentionally report `breathe dev`; official archives inject the release version at link time.

## Development

```sh
gofmt -w *.go
go test ./...
go vet ./...
go build ./...
```

## Platform and terminal expectations

Release archives target Windows AMD64, macOS ARM64 and AMD64, and Linux ARM64 and AMD64. The interactive interface expects stdin and stdout to be terminals with ANSI alternate-screen and Unicode support. Redirected or non-TTY execution uses plain output without ANSI controls or bell bytes.

## Scope and limitations

`breathe-tui` is deliberately offline and local. It has no accounts, networking, backend integration, history, persistence, configuration files, or AI features. It is a focused pacer rather than a full-screen application framework.

## License

MIT. See [LICENSE](LICENSE).
