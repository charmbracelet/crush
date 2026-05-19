package anim

import (
	"image/color"
	"strings"
	"testing"
)

func TestStaticEllipsisCycling(t *testing.T) {
	a := New(Settings{
		Static:      true,
		Size:        15,
		GradColorA:  color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff},
		GradColorB:  color.RGBA{R: 0, G: 0, B: 0xff, A: 0xff},
		LabelColor:  color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff},
		CycleColors: true,
	})

	// Capture renders for each step
	renders := make([]string, len(staticEllipsisFrames))
	for i := range staticEllipsisFrames {
		a.step.Store(int64(i))
		renders[i] = a.Render()
	}

	// Each render should contain "Working" and the appropriate dots
	for i, r := range renders {
		if !strings.Contains(r, "Working") {
			t.Errorf("expected render to contain 'Working', got %q", r)
		}
		expectedDots := staticEllipsisFrames[i]
		if expectedDots != "" && !strings.Contains(r, expectedDots) {
			t.Errorf("step %d: expected render to contain %q, got %q", i, expectedDots, r)
		}
	}

	// Verify cycle wraps correctly
	a.step.Store(int64(len(staticEllipsisFrames)))
	a.Animate(StepMsg{ID: a.id})
	if int(a.step.Load()) != 0 {
		t.Errorf("expected step to wrap to 0, got %d", a.step.Load())
	}
}

func TestStaticStartsWithWorking(t *testing.T) {
	label := color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}

	a := New(Settings{
		Static:      true,
		Size:        15,
		GradColorA:  color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff},
		GradColorB:  color.RGBA{R: 0, G: 0, B: 0xff, A: 0xff},
		LabelColor:  label,
		CycleColors: true,
	})

	// At step 0, should show "Working" (no dots yet).
	r := a.Render()
	if !strings.Contains(r, "Working") {
		t.Fatalf("expected render to contain 'Working', got %q", r)
	}
	if a.staticRendered == "" {
		t.Fatal("expected staticRendered to be set")
	}
}

func TestStaticEllipsisColor(t *testing.T) {
	label := color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
	ellipsis := color.RGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xff}

	a := New(Settings{
		Static:        true,
		Size:          15,
		LabelColor:    label,
		EllipsisColor: ellipsis,
		CycleColors:   true,
	})

	if a.ellipsisColor != ellipsis {
		t.Errorf("expected ellipsisColor to be set, got %v", a.ellipsisColor)
	}

	// When EllipsisColor is unset, it should default to LabelColor
	b := New(Settings{
		Static:      true,
		Size:        15,
		LabelColor:  label,
		CycleColors: true,
	})
	if b.ellipsisColor != label {
		t.Errorf("expected ellipsisColor to default to LabelColor, got %v", b.ellipsisColor)
	}
}
