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

	assert.Equal(t, "This is existing content in the input field", h.ExistingValue())
	assert.Equal(t, "This is existing content in the input field", h.Value())
}

func TestIntialiseHistoryScrollUp(t *testing.T) {
	fakeHistory := []string{
		"1. This is the first message",
		"2. This is the second message",
		"3. This is the third message",
	}

	h := InitialiseHistory("This is existing content in the input field", fakeHistory)
	assert.Equal(t, "This is existing content in the input field", h.ExistingValue())
	assert.Equal(t, "This is existing content in the input field", h.Value())

	h.ScrollUp()
	assert.Equal(t, "3. This is the third message", h.Value())
	h.ScrollUp()
	assert.Equal(t, "2. This is the second message", h.Value())
	h.ScrollUp()
	assert.Equal(t, "1. This is the first message", h.Value())
}

func TestIntialiseHistoryScrollDown(t *testing.T) {
	fakeHistory := []string{
		"1. This is the first message",
		"2. This is the second message",
		"3. This is the third message",
	}

	h := InitialiseHistory("This is existing content in the input field", fakeHistory)
	assert.Equal(t, "This is existing content in the input field", h.ExistingValue())
	assert.Equal(t, "This is existing content in the input field", h.Value())

	h.ScrollDown()
	assert.Equal(t, "This is existing content in the input field", h.Value())
	h.ScrollDown()
	assert.Equal(t, "This is existing content in the input field", h.Value())
	h.ScrollDown()
	assert.Equal(t, "This is existing content in the input field", h.Value())
}

func TestIntialiseHistoryScrollUpThenDown(t *testing.T) {
	fakeHistory := []string{
		"1. This is the first message",
		"2. This is the second message",
		"3. This is the third message",
	}

	h := InitialiseHistory("This is existing content in the input field", fakeHistory)
	assert.Equal(t, "This is existing content in the input field", h.ExistingValue())
	assert.Equal(t, "This is existing content in the input field", h.Value())

	h.ScrollUp()
	assert.Equal(t, "3. This is the third message", h.Value())
	h.ScrollDown()
	assert.Equal(t, "This is existing content in the input field", h.Value())
	h.ScrollUp()
	assert.Equal(t, "3. This is the third message", h.Value())
	h.ScrollUp()
	assert.Equal(t, "2. This is the second message", h.Value())
	h.ScrollDown()
	assert.Equal(t, "3. This is the third message", h.Value())
	h.ScrollDown()
	assert.Equal(t, "This is existing content in the input field", h.Value())
	h.ScrollDown()
	assert.Equal(t, "This is existing content in the input field", h.Value())
}
