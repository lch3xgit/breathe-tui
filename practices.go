package main

import (
	"fmt"
	"strings"
	"time"
)

type Phase struct {
	Name     string
	Duration time.Duration
}

type Practice struct {
	Name           string
	Slug           string
	Phases         []Phase
	TargetDuration time.Duration
	TargetCycles   int
}

func (p Practice) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("practice name is required")
	}
	if strings.TrimSpace(p.Slug) == "" {
		return fmt.Errorf("practice slug is required")
	}
	if len(p.Phases) == 0 {
		return fmt.Errorf("practice %q must have at least one phase", p.Name)
	}
	for _, phase := range p.Phases {
		if strings.TrimSpace(phase.Name) == "" {
			return fmt.Errorf("practice %q has a phase without a name", p.Name)
		}
		if phase.Duration <= 0 {
			return fmt.Errorf("practice %q phase %q must have a positive duration", p.Name, phase.Name)
		}
	}
	if p.TargetDuration < 0 || p.TargetCycles < 0 {
		return fmt.Errorf("practice %q has an invalid stopping rule", p.Name)
	}

	hasDuration := p.TargetDuration > 0
	hasCycles := p.TargetCycles > 0
	if hasDuration == hasCycles {
		return fmt.Errorf("practice %q must have exactly one stopping rule", p.Name)
	}
	return nil
}

var practices = []Practice{
	{
		Name: "Coherence", Slug: "coherence",
		Phases:         []Phase{{Name: "Inhale", Duration: 5500 * time.Millisecond}, {Name: "Exhale", Duration: 5500 * time.Millisecond}},
		TargetDuration: 5 * time.Minute,
	},
	{
		Name: "Calm", Slug: "calm",
		Phases:         []Phase{{Name: "Inhale", Duration: 4 * time.Second}, {Name: "Exhale", Duration: 6 * time.Second}},
		TargetDuration: 5 * time.Minute,
	},
	{
		Name: "Upshift", Slug: "upshift",
		Phases:         []Phase{{Name: "Inhale", Duration: 6 * time.Second}, {Name: "Exhale", Duration: 4 * time.Second}},
		TargetDuration: 5 * time.Minute,
	},
	{
		Name: "Box Breath", Slug: "box",
		Phases: []Phase{
			{Name: "Inhale", Duration: 4 * time.Second},
			{Name: "Hold full", Duration: 4 * time.Second},
			{Name: "Exhale", Duration: 4 * time.Second},
			{Name: "Hold empty", Duration: 4 * time.Second},
		},
		TargetDuration: 5 * time.Minute,
	},
	{
		Name: "Circular Flow", Slug: "circular",
		Phases:         []Phase{{Name: "Inhale", Duration: 2 * time.Second}, {Name: "Exhale", Duration: 2 * time.Second}},
		TargetDuration: 5 * time.Minute,
	},
	{
		Name: "4-7-8", Slug: "478",
		Phases: []Phase{
			{Name: "Inhale", Duration: 4 * time.Second},
			{Name: "Hold full", Duration: 7 * time.Second},
			{Name: "Exhale", Duration: 8 * time.Second},
		},
		TargetCycles: 8,
	},
}

func findPractice(slug string) (Practice, bool) {
	for _, practice := range practices {
		if practice.Slug == slug {
			return practice, true
		}
	}
	return Practice{}, false
}

func practiceSlugs() []string {
	slugs := make([]string, len(practices))
	for i, practice := range practices {
		slugs[i] = practice.Slug
	}
	return slugs
}
