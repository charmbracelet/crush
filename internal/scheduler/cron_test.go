package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseRejectsWrongFieldCount(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{"", "* * * *", "* * * * * *", "0 0"} {
		_, err := Parse(expr)
		require.Error(t, err, expr)
	}
}

func TestParseRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{
		"60 * * * *",   // minute out of range
		"* 24 * * *",   // hour out of range
		"* * 0 * *",    // day-of-month out of range
		"* * * 13 *",   // month out of range
		"* * * * 8",    // day-of-week out of range
		"a * * * *",    // not a value or a known name
		"*/0 * * * *",  // zero step
		"5-2 * * * *",  // reversed range
		"* * L * *",    // extended syntax
		", * * * *",    // empty list entry
		"1,,2 * * * *", // empty list entry
		"@daily",       // descriptors are not enabled
		"* * * * * *",  // six fields (no seconds field)
		"* * * *",      // four fields
		"",             // empty
	} {
		_, err := Parse(expr)
		require.Error(t, err, expr)
	}
}

// Month and day names, and `?` for "any", come with robfig/cron. The
// hand-rolled parser this replaced rejected them; accepting them is
// strictly more permissive, so no previously-valid expression changed
// meaning.
func TestParseAcceptsNamesAndQuestionMark(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{
		"* * * JAN *",
		"0 9 * * MON-FRI",
		"* * ? * *",
		"* * * * ?",
	} {
		_, err := Parse(expr)
		require.NoError(t, err, expr)
	}
}

func TestParseAcceptsValidExpressions(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{
		"* * * * *",
		"*/15 * * * *",
		"0 9 * * 1-5",
		"30 14 27 7 *",
		"0,30 8-18/2 1,15 * 0",
		"5/10 * * * *",
		"0 0 * * 7", // Sunday as 7
	} {
		_, err := Parse(expr)
		require.NoError(t, err, expr)
	}
}

func TestNextEveryMinute(t *testing.T) {
	t.Parallel()

	sched, err := Parse("* * * * *")
	require.NoError(t, err)

	from := time.Date(2026, 7, 27, 10, 30, 25, 0, time.Local)
	next := sched.Next(from)
	require.Equal(t, time.Date(2026, 7, 27, 10, 31, 0, 0, time.Local), next)
}

func TestNextSpecificTime(t *testing.T) {
	t.Parallel()

	sched, err := Parse("30 14 * * *")
	require.NoError(t, err)

	from := time.Date(2026, 7, 27, 10, 30, 0, 0, time.Local)
	next := sched.Next(from)
	require.Equal(t, time.Date(2026, 7, 27, 14, 30, 0, 0, time.Local), next)

	// After 14:30 the same day, the next fire is the following day.
	from = time.Date(2026, 7, 27, 14, 30, 0, 0, time.Local)
	next = sched.Next(from)
	require.Equal(t, time.Date(2026, 7, 28, 14, 30, 0, 0, time.Local), next)
}

func TestNextStepsAndRanges(t *testing.T) {
	t.Parallel()

	sched, err := Parse("*/15 9-17 * * *")
	require.NoError(t, err)

	from := time.Date(2026, 7, 27, 9, 7, 0, 0, time.Local)
	require.Equal(t, time.Date(2026, 7, 27, 9, 15, 0, 0, time.Local), sched.Next(from))

	// After 17:45 (last fire of the day) it rolls to 09:00 next day.
	from = time.Date(2026, 7, 27, 17, 45, 0, 0, time.Local)
	require.Equal(t, time.Date(2026, 7, 28, 9, 0, 0, 0, time.Local), sched.Next(from))
}

func TestNextMonthBoundary(t *testing.T) {
	t.Parallel()

	sched, err := Parse("0 9 1 3 *")
	require.NoError(t, err)

	from := time.Date(2026, 7, 27, 10, 0, 0, 0, time.Local)
	require.Equal(t, time.Date(2027, 3, 1, 9, 0, 0, 0, time.Local), sched.Next(from))
}

func TestNextDayOfWeek(t *testing.T) {
	t.Parallel()

	// 2026-07-27 is a Monday. Fire at 09:00 on Fridays only.
	sched, err := Parse("0 9 * * 5")
	require.NoError(t, err)

	from := time.Date(2026, 7, 27, 10, 0, 0, 0, time.Local)
	next := sched.Next(from)
	require.Equal(t, time.Date(2026, 7, 31, 9, 0, 0, 0, time.Local), next)
	require.Equal(t, time.Friday, next.Weekday())
}

func TestNextDayOfWeekSundayAliases(t *testing.T) {
	t.Parallel()

	zero, err := Parse("0 9 * * 0")
	require.NoError(t, err)
	seven, err := Parse("0 9 * * 7")
	require.NoError(t, err)

	from := time.Date(2026, 7, 27, 10, 0, 0, 0, time.Local) // Monday
	nextZero := zero.Next(from)
	nextSeven := seven.Next(from)
	require.Equal(t, nextZero, nextSeven)
	require.Equal(t, time.Sunday, nextZero.Weekday())
}

func TestDayOfMonthAndDayOfWeekOrSemantics(t *testing.T) {
	t.Parallel()

	// Both restricted: fires when EITHER matches. The 15th OR any Monday.
	sched, err := Parse("0 9 15 * 1")
	require.NoError(t, err)

	// From Tuesday 2026-07-14: next Monday is the 20th, but the 15th
	// comes first and matches on day-of-month alone.
	from := time.Date(2026, 7, 14, 10, 0, 0, 0, time.Local)
	next := sched.Next(from)
	require.Equal(t, time.Date(2026, 7, 15, 9, 0, 0, 0, time.Local), next)

	// From Thursday the 16th: the 15th has passed; next hit is Monday
	// the 20th on day-of-week alone.
	from = time.Date(2026, 7, 16, 10, 0, 0, 0, time.Local)
	next = sched.Next(from)
	require.Equal(t, time.Date(2026, 7, 20, 9, 0, 0, 0, time.Local), next)
}

func TestNextWildcardDayOfWeekOnly(t *testing.T) {
	t.Parallel()

	// Day-of-month restricted, day-of-week wildcard: only day-of-month
	// constrains.
	sched, err := Parse("0 9 15 * *")
	require.NoError(t, err)

	from := time.Date(2026, 7, 16, 10, 0, 0, 0, time.Local)
	require.Equal(t, time.Date(2026, 8, 15, 9, 0, 0, 0, time.Local), sched.Next(from))
}

func TestNextIsStrictlyAfterFrom(t *testing.T) {
	t.Parallel()

	sched, err := Parse("30 14 * * *")
	require.NoError(t, err)

	// Exactly on a fire time: the next fire is tomorrow, not now.
	from := time.Date(2026, 7, 27, 14, 30, 0, 0, time.Local)
	next := sched.Next(from)
	require.True(t, next.After(from))
	require.Equal(t, time.Date(2026, 7, 28, 14, 30, 0, 0, time.Local), next)
}

// withLocalTZ pins time.Local for the duration of one test so that
// Next, which evaluates schedules in the local timezone, can be tested
// against a specific zone's DST transitions.
//
// The test must not call t.Parallel: parallel tests resume only after
// the sequential pass finishes, so a sequential test body has time.Local
// to itself.
func withLocalTZ(t *testing.T, name string) {
	t.Helper()

	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	original := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = original })
}

// mustNextWithin fails the test if Next does not return within limit,
// rather than letting a non-terminating Next hang the whole suite until
// the go test timeout kills it.
func mustNextWithin(t *testing.T, sched *Schedule, from time.Time, limit time.Duration) time.Time {
	t.Helper()

	done := make(chan time.Time, 1)
	go func() { done <- sched.Next(from) }()
	select {
	case next := <-done:
		require.False(t, next.IsZero(), "schedule %q unexpectedly never fires from %v", sched, from)
		return next
	case <-time.After(limit):
		t.Fatalf("Next(%v) on %q did not return within %s", from, sched, limit)
		return time.Time{}
	}
}

// Regression test for an infinite loop that hung the scheduler.
//
// The hand-rolled parser this replaced advanced hours with
// time.Date(y, m, d, hour+1, ...). On a spring-forward day the target
// wall clock time does not exist, and Go maps it *backwards*: in
// America/New_York, time.Date(2026, 3, 8, 2, ...) returns 01:00 EST. So
// "advance past 01:00" produced 01:00 again and Next spun forever,
// burning a core while holding the store's write mutex.
//
// "0 0 * * 0" — weekly, Sunday midnight — was enough to trigger it, and
// the whole suite hung on any machine whose TZ observed DST. It passed
// in CI only because CI runs UTC.
func TestNextAcrossDSTSpringForward(t *testing.T) {
	withLocalTZ(t, "America/New_York")

	for _, tc := range []struct {
		name string
		expr string
		from time.Time
		want time.Time
	}{
		{
			name: "weekly Sunday midnight spans the transition",
			expr: "0 0 * * 0",
			from: time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local),
			want: time.Date(2026, 3, 15, 0, 0, 0, 0, time.Local),
		},
		{
			name: "hourly keeps ticking across the gap",
			expr: "0 * * * *",
			from: time.Date(2026, 3, 8, 1, 30, 0, 0, time.Local),
			want: time.Date(2026, 3, 8, 3, 0, 0, 0, time.Local),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sched, err := Parse(tc.expr)
			require.NoError(t, err)

			next := mustNextWithin(t, sched, tc.from, 10*time.Second)
			require.True(t, next.After(tc.from), "next must be strictly after from")
			require.Equal(t, tc.want.UTC(), next.UTC())
		})
	}
}

// Fall-back repeats an hour rather than skipping one. Next must still
// terminate and move strictly forward.
func TestNextAcrossDSTFallBack(t *testing.T) {
	withLocalTZ(t, "America/New_York")

	// 2026-11-01: 01:00-02:00 happens twice in America/New_York.
	sched, err := Parse("30 1 * * *")
	require.NoError(t, err)

	from := time.Date(2026, 10, 31, 12, 0, 0, 0, time.Local)
	next := mustNextWithin(t, sched, from, 10*time.Second)
	require.True(t, next.After(from))
	require.Equal(t, 1, next.Hour())
	require.Equal(t, 30, next.Minute())

	// And it keeps advancing rather than sticking on the repeated hour.
	after := mustNextWithin(t, sched, next, 10*time.Second)
	require.True(t, after.After(next), "next fire must advance past the repeated hour")
}

// Every expression the tool documents must terminate promptly from a
// spring-forward instant. This is the broad net under the specific
// regression above.
func TestNextAlwaysTerminatesOnDSTDay(t *testing.T) {
	withLocalTZ(t, "America/New_York")

	from := time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local)
	for _, expr := range []string{
		"* * * * *", "0 * * * *", "*/15 * * * *", "0 0 * * *",
		"0 0 * * 0", "0 0 * * 7", "0 0 * * 5-7", "30 2 * * *",
		"0 2 * * *", "0 9-17 * * 1-5", "0 0 1 * *", "0 0 29 2 *",
		"5/10 * * * *", "0,30 8-18/2 1,15 * 0",
	} {
		sched, err := Parse(expr)
		require.NoError(t, err, expr)

		// Walk several fires: the hang reproduced on iteration two.
		cur := from
		for i := range 5 {
			next := mustNextWithin(t, sched, cur, 10*time.Second)
			require.True(t, next.After(cur), "%s: fire %d did not advance", expr, i)
			cur = next
		}
	}
}

// An expression can be syntactically valid and still never match. Next
// reports that as the zero time; callers must not store it, since a zero
// time always compares as due.
func TestNextReturnsZeroWhenScheduleCanNeverFire(t *testing.T) {
	t.Parallel()

	sched, err := Parse("0 0 30 2 *") // February 30th
	require.NoError(t, err)

	next := sched.Next(time.Date(2026, 3, 1, 10, 30, 0, 0, time.Local))
	require.True(t, next.IsZero(), "impossible schedule must report the zero time, got %v", next)
}

func TestSundayAliasNormalization(t *testing.T) {
	t.Parallel()

	// Every pair names the same days, once via 7 and once via 0.
	for _, tc := range []struct{ sevens, zeros string }{
		{"0 0 * * 7", "0 0 * * 0"},
		{"0 0 * * 5-7", "0 0 * * 5,6,0"},
		{"0 0 * * 0-7", "0 0 * * 0-6"},
		{"0 0 * * 1,7", "0 0 * * 0,1"},
		// "n/step" means n through the end of the range. With 7 meaning
		// Sunday, 7 *is* the end, so "7/2" is just Sunday.
		{"0 0 * * 7/2", "0 0 * * 0"},
	} {
		withSeven, err := Parse(tc.sevens)
		require.NoError(t, err, tc.sevens)
		withZero, err := Parse(tc.zeros)
		require.NoError(t, err, tc.zeros)

		from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
		for range 10 {
			gotSeven := withSeven.Next(from)
			gotZero := withZero.Next(from)
			require.Equal(t, gotZero, gotSeven, "%q and %q must schedule identically", tc.sevens, tc.zeros)
			from = gotZero
		}
	}
}

// Normalizing the Sunday alias must not rewrite what the user sees:
// CronList echoes the expression back, and it round-trips through the
// durable task file.
func TestParsePreservesOriginalExpression(t *testing.T) {
	t.Parallel()

	sched, err := Parse("0 0 * * 5-7")
	require.NoError(t, err)
	require.Equal(t, "0 0 * * 5-7", sched.String())
}

// A day-of-week of * must keep star semantics, otherwise
// day-of-month/day-of-week OR matching silently changes.
func TestSundayAliasDoesNotDisturbStarSemantics(t *testing.T) {
	t.Parallel()

	// dom restricted, dow "*" => day-of-month only, no OR.
	sched, err := Parse("0 0 17 * *")
	require.NoError(t, err)

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	next := sched.Next(from)
	require.Equal(t, 17, next.Day())
	require.Equal(t, time.March, next.Month())
}
