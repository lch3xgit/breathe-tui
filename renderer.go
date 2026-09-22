package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const wideLayoutMinimum = 36

func advanceLiveFrame(session *Session, now time.Time, width int) []string {
	session.Advance(now)
	return renderCurrentFrame(session, now, width)
}

func renderCurrentFrame(session *Session, now time.Time, width int) []string {
	if session.State == SessionCompleted {
		return renderCompleted(session, width)
	}
	return renderSession(session, now, width)
}

func renderSession(session *Session, now time.Time, width int) []string {
	phase := strings.ToUpper(session.CurrentPhase.Name)
	phaseText := phase + " · " + formatCountdown(session.PhaseRemaining(now))
	metadata := sessionMetadata(session, now)
	controls := "space pause · q quit"
	if session.State == SessionPaused {
		controls = "space resume · q quit"
	}

	var lines []string
	if width >= wideLayoutMinimum {
		barSpace := width - textWidth(phaseText) - 2
		barWidth := boundedBarWidth(barSpace)
		phaseLine := phaseText
		if barWidth > 0 {
			phaseLine += "  " + progressBar(session.CurrentPhase.Name, session.PhaseProgress(now), barWidth)
		}
		lines = []string{metadata, phaseLine, controls}
	} else {
		barWidth := boundedBarWidth(width)
		bar := ""
		if barWidth > 0 {
			bar = progressBar(session.CurrentPhase.Name, session.PhaseProgress(now), barWidth)
		}
		thirdLine := metadata
		if session.State == SessionPaused {
			thirdLine = "PAUSED · " + strings.ToUpper(session.Practice.Name) + " · space resume"
		}
		lines = []string{phaseText, bar, thirdLine}
	}

	return fitLines(lines, width)
}

func renderCompleted(session *Session, width int) []string {
	return fitLines([]string{
		strings.ToUpper(session.Practice.Name),
		fmt.Sprintf("COMPLETE · %d cycles", session.CompletedCycles),
		"",
	}, width)
}

func sessionMetadata(session *Session, now time.Time) string {
	remaining := formatClock(session.SessionRemaining(now))
	var metadata string
	if session.TargetCycles > 0 {
		metadata = fmt.Sprintf("%s · breath %d/%d · %s remaining", strings.ToUpper(session.Practice.Name), session.CurrentCycle(), session.TargetCycles, remaining)
	} else {
		metadata = fmt.Sprintf("%s · %s remaining", strings.ToUpper(session.Practice.Name), remaining)
	}
	if session.State == SessionPaused {
		return "PAUSED · " + metadata
	}
	return metadata
}

func progressBar(phaseName string, progress float64, width int) string {
	if width <= 0 {
		return ""
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	var filled int
	switch phaseName {
	case "Hold full":
		filled = width
	case "Hold empty":
		filled = 0
	case "Exhale":
		filled = int(math.Round((1 - progress) * float64(width)))
	default:
		filled = int(math.Round(progress * float64(width)))
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func boundedBarWidth(available int) int {
	if available <= 0 {
		return 0
	}
	if available > 24 {
		return 24
	}
	return available
}

func formatCountdown(duration time.Duration) string {
	tenths := math.Ceil(duration.Seconds()*10) / 10
	if tenths < 0 {
		tenths = 0
	}
	return fmt.Sprintf("%.1fs", tenths)
}

func formatClock(duration time.Duration) string {
	seconds := int64(math.Ceil(duration.Seconds()))
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func fitLines(lines []string, width int) []string {
	fitted := make([]string, len(lines))
	for i, line := range lines {
		fitted[i] = truncateText(line, width)
	}
	return fitted
}

func truncateText(text string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	return string(runes[:width])
}

func textWidth(text string) int {
	return len([]rune(text))
}
