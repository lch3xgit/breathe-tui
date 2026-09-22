package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDefaultCommandSelectsCoherence(t *testing.T) {
	cmd, err := parseCommand(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.kind != commandRun || cmd.practice.Slug != "coherence" {
		t.Errorf("parseCommand(nil) = kind %v practice %q, want run/coherence", cmd.kind, cmd.practice.Slug)
	}
}

func TestSoundOptionSelectsPracticeInEitherOrder(t *testing.T) {
	tests := [][]string{
		{"--sound"},
		{"--sound", "coherence"},
		{"coherence", "--sound"},
		{"--sound", "box"},
		{"478", "--sound"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd, err := parseCommand(args)
			if err != nil {
				t.Fatal(err)
			}
			if !cmd.sound {
				t.Error("sound = false, want true")
			}
			if cmd.kind != commandRun {
				t.Errorf("kind = %v, want commandRun", cmd.kind)
			}
		})
	}
}

func TestSoundOptionDefaultsToCoherence(t *testing.T) {
	cmd, err := parseCommand([]string{"--sound"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.practice.Slug != "coherence" {
		t.Errorf("practice = %q, want coherence", cmd.practice.Slug)
	}
}

func TestPracticeCommands(t *testing.T) {
	for _, slug := range practiceSlugs() {
		t.Run(slug, func(t *testing.T) {
			cmd, err := parseCommand([]string{slug})
			if err != nil {
				t.Fatal(err)
			}
			if cmd.kind != commandRun || cmd.practice.Slug != slug {
				t.Errorf("parseCommand(%q) = kind %v practice %q, want run/%s", slug, cmd.kind, cmd.practice.Slug, slug)
			}
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"unknown"}, &stdout, &stderr)
	if code == 0 {
		t.Error("runCLI() exit code = 0, want nonzero")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	for _, text := range []string{"unknown command", "coherence", "calm", "upshift", "box", "circular", "478"} {
		if !strings.Contains(stderr.String(), text) {
			t.Errorf("stderr %q does not contain %q", stderr.String(), text)
		}
	}
}

func TestExtraArgumentsReturnUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"calm", "extra"}, &stdout, &stderr)
	if code == 0 {
		t.Error("runCLI() exit code = 0, want nonzero")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr = %q, want usage", stderr.String())
	}
}

func TestUnknownAndConflictingFlagsReturnUsageErrors(t *testing.T) {
	tests := [][]string{
		{"--unknown"},
		{"--sound", "--sound"},
		{"calm", "box"},
		{"--sound", "calm", "box"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runCLI(args, &stdout, &stderr); code == 0 {
				t.Fatalf("runCLI(%v) exit code = 0, want nonzero", args)
			}
			if !strings.Contains(stderr.String(), "Usage:") {
				t.Errorf("stderr = %q, want usage", stderr.String())
			}
		})
	}
}

func TestHelpAndVersionDoNotStartSession(t *testing.T) {
	tests := []struct {
		arg  string
		want string
	}{
		{arg: "help", want: "Usage:"},
		{arg: "--help", want: "Usage:"},
		{arg: "version", want: "breathe dev"},
		{arg: "--version", want: "breathe dev"},
	}

	for _, test := range tests {
		t.Run(test.arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runCLI([]string{test.arg}, &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), test.want)
			}
			if strings.Contains(stdout.String(), "Inhale") {
				t.Errorf("stdout = %q, appears to have started a session", stdout.String())
			}
			if strings.Contains(stdout.String(), "\x1b[") {
				t.Errorf("stdout = %q, contains ANSI cursor controls", stdout.String())
			}
			if strings.Contains(stdout.String(), "\a") {
				t.Errorf("stdout = %q, contains a terminal bell byte", stdout.String())
			}
		})
	}
}

func TestHelpDescribesTerminalBell(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{"help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "terminal bell") {
		t.Errorf("help = %q, does not describe terminal bell", stdout.String())
	}
}
