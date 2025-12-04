package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntialiseHistoryWithExistingValue(t *testing.T) {
	fakeHistory := []string{
		"1. This is the first message",
		"2. This is the second message",
		"3. This is the third message",
	}

	h := InitialiseHistory("This is existing content in the input field", fakeHistory)

	assert.Equal(t, h.ExistingValue(), "This is existing content in the input field")
	assert.Equal(t, h.Value(), "This is existing content in the input field")
}

func TestIntialiseHistoryScrollUp(t *testing.T) {
	fakeHistory := []string{
		"1. This is the first message",
		"2. This is the second message",
		"3. This is the third message",
	}

	h := InitialiseHistory("This is existing content in the input field", fakeHistory)
	assert.Equal(t, h.ExistingValue(), "This is existing content in the input field")
	assert.Equal(t, h.Value(), "This is existing content in the input field")

	h.ScrollUp()
	assert.Equal(t, h.Value(), "3. This is the third message")
	h.ScrollUp()
	assert.Equal(t, h.Value(), "2. This is the second message")
	h.ScrollUp()
	assert.Equal(t, h.Value(), "1. This is the first message")
}
