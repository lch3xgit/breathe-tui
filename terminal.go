package main

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

const wordmark = `█▄▄▄▄ ▄▄▄▄▄▄▄▄▄▄▄ ▄▄▄▄▄▄▄█▄▄▄ █▄▄▄▄ ▄▄▄▄▄
█   █ █     █▄▄▄█ ▄▄▄▄█  █    █   █ █▄▄▄█
█▄▄▄█▄█     █▄▄▄▄▄█▄▄▄█  █▄▄▄▄█   █▄█▄▄▄▄`

const (
	ansiHideCursor = "\x1b[?25l"
	ansiShowCursor = "\x1b[?25h"
	ansiReset      = "\x1b[0m"
	ansiClearLine  = "\x1b[2K"
	splashTime     = 600 * time.Millisecond
	renderInterval = 50 * time.Millisecond
)

type terminalLifecycle struct {
	writer     io.Writer
	restoreRaw func() error
	restored   bool
}

func (l *terminalLifecycle) hideCursor() error {
	_, err := io.WriteString(l.writer, ansiHideCursor)
	return err
}

func (l *terminalLifecycle) restore() error {
	if l.restored {
		return nil
	}
	l.restored = true
	_, writeErr := io.WriteString(l.writer, ansiReset+ansiShowCursor+"\r\n")
	var rawErr error
	if l.restoreRaw != nil {
		rawErr = l.restoreRaw()
	}
	return errors.Join(writeErr, rawErr)
}

type frameWriter struct {
	writer      io.Writer
	initialized bool
	lineCount   int
}

func (f *frameWriter) write(lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	var frame strings.Builder
	if f.initialized {
		frame.WriteByte('\r')
		if f.lineCount > 1 {
			frame.WriteString("\x1b[")
			frame.WriteString(strconv.Itoa(f.lineCount - 1))
			frame.WriteByte('A')
		}
	}
	for i, line := range lines {
		if f.initialized {
			frame.WriteString(ansiClearLine)
		}
		frame.WriteString(line)
		if i < len(lines)-1 {
			frame.WriteString("\r\n")
		}
	}

	_, err := io.WriteString(f.writer, frame.String())
	if err == nil {
		f.initialized = true
		f.lineCount = len(lines)
	}
	return err
}

type keyEvent struct {
	key byte
	err error
}

func interactiveTerminal(stdin, stdout *os.File) bool {
	return term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd()))
}

func runInteractiveSession(practice Practice, stdin, stdout *os.File) (handled bool, runErr error) {
	state, err := term.MakeRaw(int(stdin.Fd()))
	if err != nil {
		return false, nil
	}

	lifecycle := &terminalLifecycle{
		writer: stdout,
		restoreRaw: func() error {
			return term.Restore(int(stdin.Fd()), state)
		},
	}
	defer func() {
		runErr = errors.Join(runErr, lifecycle.restore())
	}()
	if err := lifecycle.hideCursor(); err != nil {
		return true, err
	}

	keys := readKeys(stdin)
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	frames := &frameWriter{writer: stdout}
	width, height := terminalSize(stdout)
	if wordmarkFits(width, height) {
		if err := frames.write(strings.Split(wordmark, "\n")); err != nil {
			return true, err
		}
		timer := time.NewTimer(splashTime)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				goto splashComplete
			case event := <-keys:
				if event.err != nil {
					return true, event.err
				}
				if isQuitKey(event.key) {
					return true, nil
				}
			case <-interrupts:
				return true, nil
			}
		}
	}

splashComplete:
	session, err := NewSession(practice, time.Now())
	if err != nil {
		return true, err
	}
	width, _ = terminalSize(stdout)
	if err := frames.write(renderSession(session, session.StartedAt, width)); err != nil {
		return true, err
	}

	ticker := time.NewTicker(renderInterval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			width, _ = terminalSize(stdout)
			if err := frames.write(advanceLiveFrame(session, now, width)); err != nil {
				return true, err
			}
			if session.State == SessionCompleted {
				return true, nil
			}
		case event := <-keys:
			if event.err != nil {
				return true, event.err
			}
			if isQuitKey(event.key) {
				return true, nil
			}
			if event.key == ' ' {
				now := time.Now()
				if session.State == SessionRunning {
					session.Pause(now)
				} else if session.State == SessionPaused {
					session.Resume(now)
				}
				width, _ = terminalSize(stdout)
				if err := frames.write(renderCurrentFrame(session, now, width)); err != nil {
					return true, err
				}
				if session.State == SessionCompleted {
					return true, nil
				}
			}
		case <-interrupts:
			return true, nil
		}
	}
}

func readKeys(reader io.Reader) <-chan keyEvent {
	events := make(chan keyEvent, 16)
	go func() {
		var buffer [1]byte
		for {
			n, err := reader.Read(buffer[:])
			if n > 0 {
				events <- keyEvent{key: buffer[0]}
			}
			if err != nil {
				events <- keyEvent{err: err}
				return
			}
		}
	}()
	return events
}

func isQuitKey(key byte) bool {
	return key == 'q' || key == 'Q' || key == 0x03
}

func terminalSize(output *os.File) (int, int) {
	width, height, err := term.GetSize(int(output.Fd()))
	if err != nil || width <= 0 {
		return 80, 24
	}
	return width, height
}

func wordmarkFits(width, height int) bool {
	if height < 3 {
		return false
	}
	for _, line := range strings.Split(wordmark, "\n") {
		if textWidth(line) > width {
			return false
		}
	}
	return true
}
