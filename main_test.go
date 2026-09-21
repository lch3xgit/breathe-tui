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
		})
	}
}
