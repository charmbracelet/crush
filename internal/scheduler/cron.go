package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// cronParser is a strict 5-field cron parser: minute hour day-of-month
// month day-of-week. Seconds and @descriptors (@daily, @every 1h) are
// deliberately not enabled, so the accepted syntax is exactly what the
// tool descriptions document.
//
// Parsing is delegated to robfig/cron, the de facto standard Go cron
// implementation (it is what Kubernetes uses to schedule CronJobs).
// Hand-rolling this is a trap: the previous implementation looped
// forever on daylight-saving spring-forward days because time.Date
// maps a nonexistent wall-clock hour backwards, so "advance to the
// next hour" never advanced. See TestNextAcrossDSTSpringForward.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// Schedule is a parsed 5-field cron expression evaluated in the local
// timezone. Supported syntax: wildcards (*), single values (5), steps
// (*/15), ranges (1-5), ranges with steps (1-10/2), comma lists of any
// of those, and three-letter month/day names (JAN, MON). Day-of-week
// accepts 0-7, with both 0 and 7 meaning Sunday. When both day-of-month
// and day-of-week are restricted, a time matches when either matches
// (standard cron OR semantics).
type Schedule struct {
	inner      cron.Schedule
	expression string
}

// Parse parses a strict 5-field cron expression.
func Parse(expression string) (*Schedule, error) {
	if err := rejectEmptyListEntries(expression); err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", expression, err)
	}
	inner, err := cronParser.Parse(normalizeSundayAlias(expression))
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", expression, err)
	}
	// Keep the caller's original text: it is what CronList shows back to
	// the user and what round-trips through the durable task file.
	return &Schedule{inner: inner, expression: expression}, nil
}

// rejectEmptyListEntries rejects fields with empty comma entries, such
// as "," or "1,,2".
//
// robfig/cron splits list entries with strings.FieldsFunc, which drops
// empty entries silently: "," parses cleanly into a field that matches
// nothing, and the resulting schedule never fires. Catching it here
// turns a typo into a parse error the agent can act on, instead of a
// task that is accepted and then quietly never runs.
func rejectEmptyListEntries(expression string) error {
	for _, field := range strings.Fields(expression) {
		for _, entry := range strings.Split(field, ",") {
			if strings.TrimSpace(entry) == "" {
				return fmt.Errorf("empty list entry in field %q", field)
			}
		}
	}
	return nil
}

// normalizeSundayAlias rewrites day-of-week 7 to 0 so that both spell
// Sunday, which is standard cron (and what the tool descriptions
// promise) but which robfig/cron rejects outright — its day-of-week
// domain is 0-6.
//
// The rewrite is deliberately narrow: a field with no literal 7 is
// handed to robfig untouched, so robfig keeps ownership of validation,
// error messages, names (SUN, MON), and the `*`/`?` star semantics that
// drive day-of-month/day-of-week OR matching.
func normalizeSundayAlias(expression string) string {
	fields := strings.Fields(expression)
	if len(fields) != 5 || !strings.Contains(fields[4], "7") {
		return expression
	}

	entries := strings.Split(fields[4], ",")
	days := make(map[int]bool)
	for _, entry := range entries {
		if !expandDayOfWeek(entry, days) {
			// Anything we do not fully understand goes to robfig as-is,
			// so it reports the error rather than us guessing.
			return expression
		}
	}

	normalized := make([]string, 0, len(days))
	for day := range 7 {
		if days[day] {
			normalized = append(normalized, strconv.Itoa(day))
		}
	}
	if len(normalized) == 0 {
		return expression
	}
	fields[4] = strings.Join(normalized, ",")
	return strings.Join(fields, " ")
}

// expandDayOfWeek adds the days matched by one comma-separated
// day-of-week entry to days, folding 7 onto 0. It reports whether the
// entry was understood.
func expandDayOfWeek(entry string, days map[int]bool) bool {
	step := 1
	base := entry
	if idx := strings.Index(entry, "/"); idx >= 0 {
		base = entry[:idx]
		parsed, err := strconv.Atoi(entry[idx+1:])
		if err != nil || parsed <= 0 {
			return false
		}
		step = parsed
	}

	lo, hi := 0, 7
	if base != "*" && base != "?" {
		bounds := strings.SplitN(base, "-", 2)
		parsedLo, err := strconv.Atoi(bounds[0])
		if err != nil {
			return false
		}
		lo, hi = parsedLo, parsedLo
		if len(bounds) == 2 {
			parsedHi, err := strconv.Atoi(bounds[1])
			if err != nil {
				return false
			}
			hi = parsedHi
		} else if step > 1 {
			// "5/2" means 5 through the end of the range, stepping by 2.
			hi = 7
		}
	}
	if lo < 0 || hi > 7 || lo > hi {
		return false
	}

	for v := lo; v <= hi; v += step {
		days[v%7] = true
	}
	return true
}

// String returns the expression the schedule was parsed from.
func (s *Schedule) String() string { return s.expression }

// Next returns the next time strictly after from at which the schedule
// fires, evaluated in the local timezone.
//
// It returns the zero time when the schedule can never fire again —
// either because it is impossible ("0 0 30 2 *", February 30th) or
// because the next match is more than five years out. Callers must
// check Next().IsZero(): a zero time is always "due" when compared
// against now, so storing one as a task's next run would make the task
// fire on every tick forever.
func (s *Schedule) Next(from time.Time) time.Time {
	return s.inner.Next(from.Local())
}
