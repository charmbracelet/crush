package dialog

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// TestBackgroundJobCarriesNoOutput is the guard behind the per-frame
// allocation claim.
//
// syncBuffer.String() costs a full copy of the retained window per call
// (measured 2105698 B/op, 4 allocs/op at the default 2 MiB window) — inherent to
// returning a string, not a regression. Draw and Update run on every frame
// and every keystroke, so the jobs list payload must not carry output at
// all. Adding an output field to proto.BackgroundJob would reintroduce that
// cost on every repaint (and put it on the wire on every fetch in
// client/server mode), so it fails here first.
func TestBackgroundJobCarriesNoOutput(t *testing.T) {
	want := map[string]bool{
		"ID": true, "Command": true, "Description": true,
		"StartedAt": true, "Done": true,
	}
	typ := reflect.TypeFor[proto.BackgroundJob]()
	for i := range typ.NumField() {
		require.True(t, want[typ.Field(i).Name],
			"proto.BackgroundJob gained field %q; the jobs list is fetched on open and rendered every frame, so it must stay output-free",
			typ.Field(i).Name)
	}
	require.Len(t, want, typ.NumField())
}

func benchJobs(n int) []proto.BackgroundJob {
	base := time.Now().Add(-time.Hour)
	jobs := make([]proto.BackgroundJob, 0, n)
	for i := range n {
		jobs = append(jobs, proto.BackgroundJob{
			ID:          fmt.Sprintf("%03X", i+1),
			Command:     "go test ./... -run TestSomethingRatherLongWinded",
			Description: "test run",
			StartedAt:   base.Add(time.Duration(i) * time.Second),
		})
	}
	return jobs
}

// BenchmarkJobsDialogDraw measures one repaint of the jobs dialog. Its
// allocations depend only on the number of rows drawn, never on how much
// output the underlying shells have produced — the dialog holds
// proto.BackgroundJob values, which carry none.
func BenchmarkJobsDialogDraw(b *testing.B) {
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	j, err := NewJobs(com, benchJobs(20))
	if err != nil {
		b.Fatal(err)
	}
	scr := uv.NewScreenBuffer(120, 40)
	area := uv.Rect(0, 0, 120, 40)

	b.ReportAllocs()
	for b.Loop() {
		j.Draw(scr, area)
	}
}

// BenchmarkJobsDialogRefresh measures the rebuild that arming or cancelling
// a kill triggers on the Update goroutine. It is pure list work: no IO, and
// no job output.
func BenchmarkJobsDialogRefresh(b *testing.B) {
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	j, err := NewJobs(com, benchJobs(20))
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for b.Loop() {
		j.refresh()
	}
}
