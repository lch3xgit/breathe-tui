package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const version = "dev"

type commandKind int

const (
	commandRun commandKind = iota
	commandHelp
	commandVersion
)

type command struct {
	kind     commandKind
	practice Practice
}

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	cmd, err := parseCommand(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n%s", err, usage())
		return 2
	}

	switch cmd.kind {
	case commandHelp:
		fmt.Fprint(stdout, usage())
	case commandVersion:
		fmt.Fprintf(stdout, "breathe %s\n", version)
	case commandRun:
		if err := runSession(cmd.practice, LineOutput{Writer: stdout}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}
	return 0
}

func parseCommand(args []string) (command, error) {
	if len(args) > 1 {
		return command{}, fmt.Errorf("expected zero or one argument")
	}
	if len(args) == 0 {
		practice, _ := findPractice("coherence")
		return command{kind: commandRun, practice: practice}, nil
	}

	switch args[0] {
	case "help", "--help":
		return command{kind: commandHelp}, nil
	case "version", "--version":
		return command{kind: commandVersion}, nil
	}

	practice, ok := findPractice(args[0])
	if !ok {
		return command{}, fmt.Errorf("unknown command %q; valid practices: %s", args[0], strings.Join(practiceSlugs(), ", "))
	}
	return command{kind: commandRun, practice: practice}, nil
}

func usage() string {
	return "Usage: breathe [practice|help|version]\n\nPractices: " + strings.Join(practiceSlugs(), ", ") + "\n"
}
