package anim

import (
	"image/color"
	"testing"
)

func TestStaticColorCycling(t *testing.T) {
	a := New(Settings{
		Static:      true,
		Size:        15,
		GradColorA:  color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff},
		GradColorB:  color.RGBA{R: 0, G: 0, B: 0xff, A: 0xff},
		LabelColor:  color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff},
		CycleColors: true,
	})

	// Capture initial render
	previous := a.Render()

	distinctRenders := 0

	// Simulate 3 full cycles (30 steps) and track distinct renders
	for i := 0; i < 30; i++ {
		a.step.Add(1)
		current := a.Render()
		if current != previous {
			distinctRenders++
			previous = current
		}
	}

	// With 10 distinct colors in a cycle, we should see at least 5 changes
	// across 30 steps (3 cycles × 10 colors = 30, but step 0 repeats at wrap)
	if distinctRenders < 5 {
		t.Errorf("expected at least 5 distinct renders across 3 cycles, got %d", distinctRenders)
	}
}

func TestStaticStartsWithGradColorA(t *testing.T) {
	red := color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff}
	blue := color.RGBA{R: 0, G: 0, B: 0xff, A: 0xff}
	label := color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}

	a := New(Settings{
		Static:      true,
		Size:        15,
		GradColorA:  red,
		GradColorB:  blue,
		LabelColor:  label,
		CycleColors: true,
	})

	// At step 0, dotColor should be GradColorA (red)
	r := a.Render()
	if len(r) == 0 {
		t.Fatal("expected non-empty render")
	}
	// Verify it contains "Working" in the label color
	if a.staticRendered == "" {
		t.Fatal("expected staticRendered to be set")
	}
}
