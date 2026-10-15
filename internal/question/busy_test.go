package question

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Hai client cùng huỷ một câu hỏi: chỉ một lần đóng kênh (trước đây đóng hai
// lần là panic trong handler), đúng một lời gọi báo true.
func TestCancelFromTwoClientsDoesNotPanic(t *testing.T) {
	t.Parallel()
	svc := NewService()
	done := make(chan error, 1)
	go func() {
		_, err := svc.Ask(t.Context(), freeTextRequest("s1"))
		done <- err
	}()
	require.Eventually(t, func() bool { _, ok := svc.Pending(); return ok }, 2*time.Second, 5*time.Millisecond)

	var (
		wg   sync.WaitGroup
		wins atomic.Int32
	)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if svc.Cancel() {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), wins.Load())
	require.ErrorIs(t, <-done, ErrCancelled)
}

// Câu hỏi của session khác đang chờ trả lời (session không ai xem): câu hỏi
// sau không chờ vô hình mãi, mà trả ErrQuestionBusy sau askQueueTimeout.
func TestAskGivesUpBehindAnotherSessionsQuestion(t *testing.T) {
	// Không parallel: đổi askQueueTimeout.
	prev := askQueueTimeout
	askQueueTimeout = 100 * time.Millisecond
	t.Cleanup(func() { askQueueTimeout = prev })

	svc := NewService()
	go func() { _, _ = svc.Ask(t.Context(), freeTextRequest("unattended")) }()
	require.Eventually(t, func() bool { _, ok := svc.Pending(); return ok }, 2*time.Second, 5*time.Millisecond)

	start := time.Now()
	_, err := svc.Ask(t.Context(), freeTextRequest("watched"))
	require.ErrorIs(t, err, ErrQuestionBusy)
	require.Less(t, time.Since(start), 2*time.Second)
}
