package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTitleAnimation(t *testing.T) {
	t.Parallel()

	m := &UI{}
	cmd := m.startTitleAnimation("Hi")
	require.NotNil(t, cmd)
	require.True(t, m.titleAnim.active)
	require.Equal(t, titlePhaseBlink, m.titleAnim.phase)
	require.True(t, m.titleAnim.cursorOn)

	gen := m.titleAnim.gen

	// Blink phase: cursor toggles, counting a blink each time it goes dark.
	// Two blinks means two dark toggles before streaming begins.
	m.handleTitleAnimTick(titleAnimTickMsg{gen: gen}) // -> dark, blinks=1
	require.Equal(t, titlePhaseBlink, m.titleAnim.phase)
	require.False(t, m.titleAnim.cursorOn)
	require.Equal(t, 1, m.titleAnim.blinks)

	m.handleTitleAnimTick(titleAnimTickMsg{gen: gen}) // -> on
	require.Equal(t, titlePhaseBlink, m.titleAnim.phase)
	require.True(t, m.titleAnim.cursorOn)

	m.handleTitleAnimTick(titleAnimTickMsg{gen: gen}) // -> dark, blinks=2 -> stream
	require.Equal(t, titlePhaseStream, m.titleAnim.phase)
	require.Equal(t, 0, m.titleAnim.revealed)

	// Stream phase reveals one rune per tick.
	m.handleTitleAnimTick(titleAnimTickMsg{gen: gen})
	require.Equal(t, 1, m.titleAnim.revealed)
	require.True(t, m.titleAnim.active)

	m.handleTitleAnimTick(titleAnimTickMsg{gen: gen})
	require.Equal(t, 2, m.titleAnim.revealed)

	// All runes revealed; the next tick ends the animation.
	cmd = m.handleTitleAnimTick(titleAnimTickMsg{gen: gen})
	require.Nil(t, cmd)
	require.False(t, m.titleAnim.active)
}

func TestTitleAnimationIgnoresStaleTicks(t *testing.T) {
	t.Parallel()

	m := &UI{}
	m.startTitleAnimation("first")
	staleGen := m.titleAnim.gen

	// A new animation supersedes the old one.
	m.startTitleAnimation("second")
	require.Equal(t, "second", m.titleAnim.target)

	// Ticks from the superseded animation must be ignored.
	before := m.titleAnim
	require.Nil(t, m.handleTitleAnimTick(titleAnimTickMsg{gen: staleGen}))
	require.Equal(t, before, m.titleAnim)
}
