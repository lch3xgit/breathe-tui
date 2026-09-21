package main

import (
	"fmt"
	"io"
	"time"
)

type SessionState int

const (
	SessionRunning SessionState = iota
	SessionCompleted
)

type PhaseStart struct {
	Phase     Phase
	Index     int
	StartedAt time.Time
	Deadline  time.Time
}

type Session struct {
	Practice          Practice
	CurrentPhase      Phase
	CurrentPhaseIndex int
	CompletedCycles   int
	PhaseStartedAt    time.Time
	PhaseDeadline     time.Time
	TargetDuration    time.Duration
	TargetCycles      int
	StartedAt         time.Time
	State             SessionState
}

func NewSession(practice Practice, startedAt time.Time) (*Session, error) {
	if err := practice.Validate(); err != nil {
		return nil, err
	}

	first := practice.Phases[0]
	return &Session{
		Practice:          practice,
		CurrentPhase:      first,
		CurrentPhaseIndex: 0,
		PhaseStartedAt:    startedAt,
		PhaseDeadline:     startedAt.Add(first.Duration),
		TargetDuration:    practice.TargetDuration,
		TargetCycles:      practice.TargetCycles,
		StartedAt:         startedAt,
		State:             SessionRunning,
	}, nil
}

func (s *Session) Advance(now time.Time) []PhaseStart {
	var starts []PhaseStart
	for s.State == SessionRunning && !now.Before(s.PhaseDeadline) {
		boundary := s.PhaseDeadline
		if s.CurrentPhaseIndex == len(s.Practice.Phases)-1 {
			s.CompletedCycles++
			if s.reachedTarget(boundary) {
				s.State = SessionCompleted
				break
			}
			s.CurrentPhaseIndex = 0
		} else {
			s.CurrentPhaseIndex++
		}

		s.CurrentPhase = s.Practice.Phases[s.CurrentPhaseIndex]
		s.PhaseStartedAt = boundary
		s.PhaseDeadline = boundary.Add(s.CurrentPhase.Duration)
		starts = append(starts, PhaseStart{
			Phase:     s.CurrentPhase,
			Index:     s.CurrentPhaseIndex,
			StartedAt: s.PhaseStartedAt,
			Deadline:  s.PhaseDeadline,
		})
	}
	return starts
}

func (s *Session) reachedTarget(cycleBoundary time.Time) bool {
	if s.TargetCycles > 0 {
		return s.CompletedCycles >= s.TargetCycles
	}
	return cycleBoundary.Sub(s.StartedAt) >= s.TargetDuration
}

type SessionOutput interface {
	Started(Practice)
	PhaseStarted(PhaseStart)
	Completed(Practice, int)
}

type LineOutput struct {
	Writer io.Writer
}

func (o LineOutput) Started(practice Practice) {
	if practice.TargetCycles > 0 {
		fmt.Fprintf(o.Writer, "%s — %d cycles\n", practice.Name, practice.TargetCycles)
		return
	}
	fmt.Fprintf(o.Writer, "%s — %s target; the final cycle will be completed\n", practice.Name, formatDuration(practice.TargetDuration))
}

func (o LineOutput) PhaseStarted(start PhaseStart) {
	fmt.Fprintf(o.Writer, "%s — %s\n", start.Phase.Name, formatDuration(start.Phase.Duration))
}

func (o LineOutput) Completed(practice Practice, cycles int) {
	fmt.Fprintf(o.Writer, "%s complete — %d cycles\n", practice.Name, cycles)
}

func formatDuration(duration time.Duration) string {
	if duration%time.Second == 0 {
		return fmt.Sprintf("%d seconds", duration/time.Second)
	}
	return fmt.Sprintf("%.1f seconds", duration.Seconds())
}

func runSession(practice Practice, output SessionOutput) error {
	now := time.Now()
	session, err := NewSession(practice, now)
	if err != nil {
		return err
	}

	output.Started(practice)
	output.PhaseStarted(PhaseStart{
		Phase:     session.CurrentPhase,
		Index:     session.CurrentPhaseIndex,
		StartedAt: session.PhaseStartedAt,
		Deadline:  session.PhaseDeadline,
	})
	for session.State == SessionRunning {
		timer := time.NewTimer(time.Until(session.PhaseDeadline))
		<-timer.C
		for _, start := range session.Advance(time.Now()) {
			output.PhaseStarted(start)
		}
	}
	output.Completed(practice, session.CompletedCycles)
	return nil
}
