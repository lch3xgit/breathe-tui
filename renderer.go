package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	phaseLabelWidth             = len("HOLD EMPTY")
	countdownWidth              = len("8.0s")
	preferredMeterInteriorWidth = 20
	meterBoundaryWidth          = 2
	meterPaddingWidth           = 2
	meterFixedWidth             = meterBoundaryWidth + meterPaddingWidth
	preferredFrameWidth         = 52
	framedMinimum               = 28
)

func advanceLiveFrame(session *Session, now time.Time, width int) []string {
	session.Advance(now)
	return renderCurrentFrame(session, now, width)
}

func renderCurrentFrame(session *Session, now time.Time, width int) []string {
	width = effectiveFrameWidth(width)
	if session.State == SessionCompleted {
		return renderCompleted(session, width)
	}
	return renderSession(session, now, width)
}

func renderSession(session *Session, now time.Time, width int) []string {
	width = effectiveFrameWidth(width)
	header := sessionMetadata(session, now)
	controls := "space pause · q quit"
	if session.State == SessionPaused {
		controls = "space resume · q quit"
	}

	if width < framedMinimum {
		return renderNarrowFrame(session, now, width, header, controls)
	}

	return fitLines([]string{
		framedHeader(header, width),
		framedMiddle(session, now, width),
		framedFooter(controls, width),
	}, width)
}

func renderCompleted(session *Session, width int) []string {
	width = effectiveFrameWidth(width)
	return fitLines([]string{
		framedHeader(strings.ToUpper(session.Practice.Name)+" · complete", width),
		"│ " + fmt.Sprintf("complete · %s", formatClock(session.ScheduledEnd().Sub(session.StartedAt))),
		framedFooter("q quit", width),
	}, width)
}

func renderNarrowFrame(session *Session, now time.Time, width int, header, controls string) []string {
	phaseCue := compactPhaseCue(session, now)
	if width <= 0 {
		return []string{"", "", ""}
	}
	if width == 1 {
		return []string{"┌", "│", "└"}
	}
	if width < 2+phaseLabelWidth+3+countdownWidth {
		return fitLines([]string{
			"┌─" + compactHeader(header),
			"│ " + phaseCue,
			"└─" + compactControls(controls),
		}, width)
	}
	return fitLines([]string{
		framedHeader(compactHeader(header), width),
		"│ " + phaseCue,
		framedFooter(compactControls(controls), width),
	}, width)
}

func framedHeader(header string, width int) string {
	if width <= 0 {
		return ""
	}
	if width == 1 {
		return "┌"
	}
	prefix := "┌─"
	if width == 2 {
		return prefix
	}

	space := width - textWidth(prefix) - 1
	content := truncateText(header, space)
	line := prefix + " " + content
	remaining := width - textWidth(line)
	if remaining > 0 {
		if remaining == 1 {
			line += "─"
		} else {
			line += " " + strings.Repeat("─", remaining-1)
		}
	}
	return line
}

func framedMiddle(session *Session, now time.Time, width int) string {
	interiorWidth := boundedMeterInteriorWidth(width - middleFixedWidth())
	return "│ " + breathMeter(session.CurrentPhase.Name, session.PhaseProgress(now), interiorWidth) + " · " + phaseCue(session, now)
}

func framedFooter(controls string, width int) string {
	if width <= 0 {
		return ""
	}
	if width == 1 {
		return "└"
	}
	return truncateText("└─ "+controls, width)
}

func phaseCue(session *Session, now time.Time) string {
	label := strings.ToUpper(session.CurrentPhase.Name)
	return formatCountdown(session.PhaseRemaining(now)) + " · " + label
}

func compactPhaseCue(session *Session, now time.Time) string {
	return strings.ToUpper(session.CurrentPhase.Name) + " · " + formatCountdown(session.PhaseRemaining(now))
}

func middleFixedWidth() int {
	return textWidth("│ ") + meterFixedWidth + textWidth(" · ") + countdownWidth + textWidth(" · ") + phaseLabelWidth
}

func effectiveFrameWidth(width int) int {
	if width > preferredFrameWidth {
		return preferredFrameWidth
	}
	return width
}

func sessionMetadata(session *Session, now time.Time) string {
	remaining := formatClock(session.SessionRemaining(now))
	name := strings.ToUpper(session.Practice.Name)
	if session.TargetCycles > 0 {
		if session.State == SessionPaused {
			return fmt.Sprintf("%s · PAUSED · breath %d/%d · %s remaining", name, session.CurrentCycle(), session.TargetCycles, remaining)
		}
		return fmt.Sprintf("%s · breath %d/%d · %s remaining", name, session.CurrentCycle(), session.TargetCycles, remaining)
	}
	if session.State == SessionPaused {
		return fmt.Sprintf("%s · PAUSED · %s remaining", name, remaining)
	}
	return fmt.Sprintf("%s · %s remaining", name, remaining)
}

func compactHeader(header string) string {
	return strings.Replace(header, " remaining", "", 1)
}

func compactControls(controls string) string {
	return strings.Replace(controls, "space ", "", 1)
}

func breathMeter(phaseName string, progress float64, interiorWidth int) string {
	if interiorWidth <= 0 {
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
		filled = interiorWidth
	case "Hold empty":
		filled = 0
	case "Exhale":
		filled = int(math.Round((1 - progress) * float64(interiorWidth)))
	default:
		filled = int(math.Round(progress * float64(interiorWidth)))
	}
	if filled < 0 {
		filled = 0
	}
	if filled > interiorWidth {
		filled = interiorWidth
	}
	return "[ " + strings.Repeat("░", filled) + strings.Repeat(" ", interiorWidth-filled) + " ]"
}

func boundedMeterInteriorWidth(available int) int {
	if available <= 0 {
		return 0
	}
	if available > preferredMeterInteriorWidth {
		return preferredMeterInteriorWidth
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

func sessionSummary(session *Session, now time.Time, completed bool) string {
	name := strings.ToUpper(session.Practice.Name)
	if completed {
		return fmt.Sprintf("breathe · %s · complete · %s", name, formatClock(session.ScheduledEnd().Sub(session.StartedAt)))
	}
	elapsed := session.activeTime(now).Sub(session.StartedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	return fmt.Sprintf("breathe · %s · ended after %s", name, formatClock(elapsed))
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
