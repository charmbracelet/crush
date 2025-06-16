package anim

import (
	"testing"
	"time"
)

func BenchmarkAnimationUpdate(b *testing.B) {
	anim := New(30, "Loading test data")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate character cycling update
		_, _ = anim.Update(StepCharsMsg{id: anim.ID()})
	}
}

func BenchmarkAnimationView(b *testing.B) {
	anim := New(30, "Loading test data")
	
	// Initialize with some cycling
	for i := 0; i < 10; i++ {
		anim.Update(StepCharsMsg{id: anim.ID()})
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = anim.View()
	}
}

func BenchmarkRandomRune(b *testing.B) {
	c := cyclingChar{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.randomRune()
	}
}

func TestAnimationPerformance(t *testing.T) {
	anim := New(30, "Performance test")
	
	start := time.Now()
	iterations := 1000
	
	// Simulate rapid updates
	for i := 0; i < iterations; i++ {
		anim.Update(StepCharsMsg{id: anim.ID()})
		_ = anim.View()
	}
	
	duration := time.Since(start)
	avgPerUpdate := duration / time.Duration(iterations)
	
	// Should complete 1000 updates in reasonable time
	if avgPerUpdate > time.Millisecond {
		t.Errorf("Animation update too slow: %v per update (should be < 1ms)", avgPerUpdate)
	}
	
	t.Logf("Animation performance: %v per update (%d updates in %v)", 
		avgPerUpdate, iterations, duration)
}