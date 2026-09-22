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
	sound    bool
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
		if terminalOutput, ok := stdout.(*os.File); ok && interactiveTerminal(os.Stdin, terminalOutput) {
			handled, err := runInteractiveSession(cmd.practice, os.Stdin, terminalOutput, cmd.sound)
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			if handled {
				return 0
			}
		}
		if err := runSession(cmd.practice, LineOutput{Writer: stdout}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}
	return 0
}

func parseCommand(args []string) (command, error) {
	var practice Practice
	var hasPractice, sound bool
	for _, arg := range args {
		switch arg {
		case "help", "--help":
			if len(args) != 1 {
				return command{}, fmt.Errorf("help cannot be combined with other arguments")
			}
			return command{kind: commandHelp}, nil
		case "version", "--version":
			if len(args) != 1 {
				return command{}, fmt.Errorf("version cannot be combined with other arguments")
			}
			return command{kind: commandVersion}, nil
		case "--sound":
			if sound {
				return command{}, fmt.Errorf("--sound may be specified only once")
			}
			sound = true
		default:
			if strings.HasPrefix(arg, "-") {
				return command{}, fmt.Errorf("unknown flag %q", arg)
			}
			if hasPractice {
				return command{}, fmt.Errorf("expected at most one practice")
			}
			var ok bool
			practice, ok = findPractice(arg)
			if !ok {
				return command{}, fmt.Errorf("unknown command %q; valid practices: %s", arg, strings.Join(practiceSlugs(), ", "))
			}
			hasPractice = true
		}
	}
	if !hasPractice {
		practice, _ = findPractice("coherence")
	}
	return command{kind: commandRun, practice: practice, sound: sound}, nil
}

func usage() string {
	return "Usage: breathe [--sound] [practice]\n       breathe help|version\n\nPractices: " + strings.Join(practiceSlugs(), ", ") + "\n\n--sound rings the terminal bell at live phase changes; terminal settings control whether it is audible.\n"
}
