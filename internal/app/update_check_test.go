package app

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/update"
	"github.com/stretchr/testify/require"
)

// fakeUpdateClient thay update.Default: đếm số lần "gọi GitHub" và trả bản mới
// nhất cho trước (rỗng = lỗi mạng).
type fakeUpdateClient struct {
	calls  atomic.Int32
	latest string
}

func (c *fakeUpdateClient) Latest(context.Context) (*update.Release, error) {
	c.calls.Add(1)
	if c.latest == "" {
		return nil, errors.New("offline")
	}
	return &update.Release{TagName: c.latest, HTMLURL: "https://example.invalid/release"}, nil
}

// useFakeUpdateClient thay client kiểm tra bản mới toàn tiến trình bằng bản giả
// (khôi phục khi test xong). Không dùng cùng t.Parallel().
func useFakeUpdateClient(t *testing.T, latest string) *fakeUpdateClient {
	t.Helper()
	fake := &fakeUpdateClient{latest: latest}
	orig := update.Default
	update.Default = fake
	t.Cleanup(func() { update.Default = orig })
	return fake
}

// setUpdateCheckEnv đặt CRUSH_DISABLE_UPDATE_CHECK (set=false nghĩa là bỏ hẳn
// biến) và khôi phục giá trị cũ khi test xong.
func setUpdateCheckEnv(t *testing.T, value string, set bool) {
	t.Helper()
	t.Setenv("CRUSH_DISABLE_UPDATE_CHECK", value) // đăng ký việc khôi phục
	if !set {
		require.NoError(t, os.Unsetenv("CRUSH_DISABLE_UPDATE_CHECK"))
	}
}

// Fork mặc định không gọi GitHub; chỉ giá trị false tường minh (theo
// strconv.ParseBool) mới bật. "0" và "false" từng bị coi là TẮT vì mọi giá trị
// khác rỗng đều tắt.
func TestCheckForUpdates_CallsGitHubOnlyWhenExplicitlyEnabled(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		set     bool
		enabled bool
	}{
		{"unset (fork default)", "", false, false},
		{"empty", "", true, false},
		{"1", "1", true, false},
		{"true", "true", true, false},
		{"TRUE", "TRUE", true, false},
		{"t", "t", true, false},
		{"unparsable word", "maybe", true, false},
		{"yes is not a ParseBool value", "yes", true, false},
		{"no is not a ParseBool value", "no", true, false},
		{"0", "0", true, true},
		{"false", "false", true, true},
		{"False", "False", true, true},
		{"FALSE", "FALSE", true, true},
		{"f", "f", true, true},
		{"F", "F", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := useFakeUpdateClient(t, "")
			setUpdateCheckEnv(t, tc.value, tc.set)

			require.Equal(t, tc.enabled, updateCheckEnabled())
			(&App{}).checkForUpdates(t.Context())

			want := int32(0)
			if tc.enabled {
				want = 1
			}
			require.Equal(t, want, fake.calls.Load(), "GitHub calls")
		})
	}
}

// Khi bật tường minh và có bản mới, thông báo vẫn tới được bus sự kiện như cũ.
func TestCheckForUpdates_PublishesWhenEnabledAndNewer(t *testing.T) {
	fake := useFakeUpdateClient(t, "v999.0.0")
	setUpdateCheckEnv(t, "false", true)

	events := pubsub.NewBroker[tea.Msg]()
	defer events.Shutdown()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ch := events.Subscribe(ctx)

	(&App{events: events}).checkForUpdates(ctx)
	require.Equal(t, int32(1), fake.calls.Load())

	select {
	case ev := <-ch:
		msg, ok := ev.Payload.(UpdateAvailableMsg)
		require.True(t, ok, "payload %T", ev.Payload)
		require.Equal(t, "999.0.0", msg.LatestVersion)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the update notification")
	}
}
