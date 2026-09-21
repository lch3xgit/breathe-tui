package main

import (
	"reflect"
	"testing"
	"time"
)

func TestBuiltInPractices(t *testing.T) {
	want := map[string]struct {
		name           string
		phases         []Phase
		targetDuration time.Duration
		targetCycles   int
	}{
		"coherence": {
			name: "Coherence", phases: []Phase{{"Inhale", 5500 * time.Millisecond}, {"Exhale", 5500 * time.Millisecond}}, targetDuration: 5 * time.Minute,
		},
		"calm": {
			name: "Calm", phases: []Phase{{"Inhale", 4 * time.Second}, {"Exhale", 6 * time.Second}}, targetDuration: 5 * time.Minute,
		},
		"upshift": {
			name: "Upshift", phases: []Phase{{"Inhale", 6 * time.Second}, {"Exhale", 4 * time.Second}}, targetDuration: 5 * time.Minute,
		},
		"box": {
			name: "Box Breath", phases: []Phase{{"Inhale", 4 * time.Second}, {"Hold full", 4 * time.Second}, {"Exhale", 4 * time.Second}, {"Hold empty", 4 * time.Second}}, targetDuration: 5 * time.Minute,
		},
		"circular": {
			name: "Circular Flow", phases: []Phase{{"Inhale", 2 * time.Second}, {"Exhale", 2 * time.Second}}, targetDuration: 5 * time.Minute,
		},
		"478": {
			name: "4-7-8", phases: []Phase{{"Inhale", 4 * time.Second}, {"Hold full", 7 * time.Second}, {"Exhale", 8 * time.Second}}, targetCycles: 8,
		},
	}

	if len(practices) != len(want) {
		t.Fatalf("len(practices) = %d, want %d", len(practices), len(want))
	}
	for slug, expected := range want {
		practice, ok := findPractice(slug)
		if !ok {
			t.Errorf("findPractice(%q) did not find a practice", slug)
			continue
		}
		if practice.Name != expected.name {
			t.Errorf("%s name = %q, want %q", slug, practice.Name, expected.name)
		}
		if !reflect.DeepEqual(practice.Phases, expected.phases) {
			t.Errorf("%s phases = %#v, want %#v", slug, practice.Phases, expected.phases)
		}
		if practice.TargetDuration != expected.targetDuration || practice.TargetCycles != expected.targetCycles {
			t.Errorf("%s stopping rule = (%s, %d), want (%s, %d)", slug, practice.TargetDuration, practice.TargetCycles, expected.targetDuration, expected.targetCycles)
		}
		if err := practice.Validate(); err != nil {
			t.Errorf("%s validation failed: %v", slug, err)
		}
	}

	if _, ok := findPractice("not-a-practice"); ok {
		t.Error("findPractice found an unknown slug")
	}
	if got, want := practiceSlugs(), []string{"coherence", "calm", "upshift", "box", "circular", "478"}; !reflect.DeepEqual(got, want) {
		t.Errorf("practiceSlugs() = %v, want %v", got, want)
	}
}

func TestPracticePhaseOrdering(t *testing.T) {
	coherence, _ := findPractice("coherence")
	if got, want := phaseNames(coherence), []string{"Inhale", "Exhale"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Coherence phases = %v, want %v", got, want)
	}

	box, _ := findPractice("box")
	if got, want := phaseNames(box), []string{"Inhale", "Hold full", "Exhale", "Hold empty"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Box phases = %v, want %v", got, want)
	}
}

func TestCoherenceUsesExactFivePointFiveSecondPhases(t *testing.T) {
	practice, _ := findPractice("coherence")
	for _, phase := range practice.Phases {
		if phase.Duration != 5500*time.Millisecond {
			t.Errorf("%s duration = %s, want 5.5s", phase.Name, phase.Duration)
		}
	}
}

func TestPracticeValidationStopRules(t *testing.T) {
	validPhase := []Phase{{Name: "Inhale", Duration: time.Second}}
	tests := []struct {
		name     string
		practice Practice
	}{
		{
			name:     "missing stop rule",
			practice: Practice{Name: "Test", Slug: "test", Phases: validPhase},
		},
		{
			name: "conflicting stop rules",
			practice: Practice{
				Name: "Test", Slug: "test", Phases: validPhase,
				TargetDuration: time.Minute, TargetCycles: 1,
			},
		},
		{
			name: "negative stop rule",
			practice: Practice{
				Name: "Test", Slug: "test", Phases: validPhase,
				TargetDuration: -time.Second, TargetCycles: 1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.practice.Validate(); err == nil {
				t.Error("Validate() returned nil, want an error")
			}
		})
	}
}

func phaseNames(practice Practice) []string {
	names := make([]string, len(practice.Phases))
	for i, phase := range practice.Phases {
		names[i] = phase.Name
	}
	return names
}
