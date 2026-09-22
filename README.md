# breathe-tui

`breathe-tui` is a tiny, fast terminal breath pacer written in Go. It is in early development and includes a compact interactive terminal display.

## Practices

Run one of six built-in practices:

- `coherence` — inhale 5.5 seconds, exhale 5.5 seconds
- `calm` — inhale 4 seconds, exhale 6 seconds
- `upshift` — inhale 6 seconds, exhale 4 seconds
- `box` — inhale, hold full, exhale, and hold empty for 4 seconds each
- `circular` — inhale 2 seconds, exhale 2 seconds
- `478` — inhale 4 seconds, hold full 7 seconds, exhale 8 seconds

Running without a practice selects Coherence. The first five practices target five minutes. When that target is reached during a breath cycle, the current cycle finishes before the session ends; a new cycle is not started after the target has been met. The 4-7-8 practice instead runs for exactly eight complete cycles.

## Run locally

Go 1.23 or later is required.

```sh
go run .
go run . calm
go run . --sound box
go run . help
```

In an interactive terminal, the pacer temporarily uses a dedicated terminal screen. When space permits, its wordmark remains above a compact three-line frame with a bracketed breath meter. Press Space to pause or resume, and `q` to quit. Ctrl+C also exits. The normal shell screen is restored before a one-line session summary is printed. `--sound` emits a terminal bell at live phase changes; whether that becomes an audible click depends on terminal settings. Redirected or non-terminal execution retains simple line-oriented output without ANSI cursor controls or bell bytes.

The project is deliberately small and offline. Its only external dependency is `golang.org/x/term`, used for raw terminal input, terminal detection, state restoration, and terminal dimensions. It has no accounts, network services, persistence, or connection to the GoodBeet/Breathe web application. Packaging an installed `breathe` command remains future work.
