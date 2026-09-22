package main

import (
	"fmt"
	"io"
	"time"
)

type SessionState int

const (
	SessionRunning SessionState = iota
	SessionPaused
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
	PausedAt          time.Time
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

func (s *Session) Pause(now time.Time) []PhaseStart {
	if s.State != SessionRunning {
		return nil
	}

	starts := s.Advance(now)
	if s.State == SessionRunning {
		s.PausedAt = now
		s.State = SessionPaused
	}
	return starts
}

func (s *Session) Resume(now time.Time) bool {
	if s.State != SessionPaused || now.Before(s.PausedAt) {
		return false
	}

	pauseDuration := now.Sub(s.PausedAt)
	s.StartedAt = s.StartedAt.Add(pauseDuration)
	s.PhaseStartedAt = s.PhaseStartedAt.Add(pauseDuration)
	s.PhaseDeadline = s.PhaseDeadline.Add(pauseDuration)
	s.PausedAt = time.Time{}
	s.State = SessionRunning
	return true
}

func (s *Session) PhaseRemaining(now time.Time) time.Duration {
	if s.State == SessionCompleted {
		return 0
	}
	remaining := s.PhaseDeadline.Sub(s.activeTime(now))
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s *Session) PhaseProgress(now time.Time) float64 {
	elapsed := s.activeTime(now).Sub(s.PhaseStartedAt)
	progress := float64(elapsed) / float64(s.CurrentPhase.Duration)
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

func (s *Session) ScheduledEnd() time.Time {
	cycleDuration := s.cycleDuration()
	cycles := s.TargetCycles
	if cycles == 0 {
		cycles = int(s.TargetDuration / cycleDuration)
		if s.TargetDuration%cycleDuration != 0 {
			cycles++
		}
	}
	return s.StartedAt.Add(time.Duration(cycles) * cycleDuration)
}

func (s *Session) SessionRemaining(now time.Time) time.Duration {
	if s.State == SessionCompleted {
		return 0
	}
	remaining := s.ScheduledEnd().Sub(s.activeTime(now))
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s *Session) CurrentCycle() int {
	if s.State == SessionCompleted {
		return s.CompletedCycles
	}
	return s.CompletedCycles + 1
}

func (s *Session) activeTime(now time.Time) time.Time {
	if s.State == SessionPaused {
		return s.PausedAt
	}
	return now
}

func (s *Session) cycleDuration() time.Duration {
	var duration time.Duration
	for _, phase := range s.Practice.Phases {
		duration += phase.Duration
	}
	return duration
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
