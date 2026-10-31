package mcpoauth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type tokenSourceFunc func() (*oauth2.Token, error)

func (f tokenSourceFunc) Token() (*oauth2.Token, error) { return f() }

func TestSavingTokenSource_SerializesRefreshAndSave(t *testing.T) {
	t.Parallel()

	firstSave := make(chan struct{})
	releaseSave := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseSave) }) }
	t.Cleanup(release)

	var calls atomic.Int32
	source := tokenSourceFunc(func() (*oauth2.Token, error) {
		if calls.Add(1) == 1 {
			return &oauth2.Token{AccessToken: "first", RefreshToken: "first-refresh"}, nil
		}
		return &oauth2.Token{AccessToken: "second", RefreshToken: "second-refresh"}, nil
	})
	var savedMu sync.Mutex
	var saved []string
	ts := NewSavingTokenSource(source, nil, nil, func(_ *oauth2.Config, tok *oauth2.Token) {
		if tok.AccessToken == "first" {
			close(firstSave)
			<-releaseSave
		}
		savedMu.Lock()
		saved = append(saved, tok.RefreshToken)
		savedMu.Unlock()
	})

	firstDone := make(chan error, 1)
	go func() {
		_, err := ts.Token()
		firstDone <- err
	}()
	<-firstSave

	secondDone := make(chan error, 1)
	go func() {
		_, err := ts.Token()
		secondDone <- err
	}()
	secondFinished := false
	select {
	case err := <-secondDone:
		secondFinished = true
		require.NoError(t, err)
		t.Error("a later token refresh completed before the previous token was saved")
	case <-time.After(100 * time.Millisecond):
	}
	release()
	require.NoError(t, <-firstDone)
	if !secondFinished {
		require.NoError(t, <-secondDone)
	}
	savedMu.Lock()
	defer savedMu.Unlock()
	require.Equal(t, []string{"first-refresh", "second-refresh"}, saved)
}
