// Package julianday provides Time to Julian day conversions.
package julianday

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"time"
)

const secs_per_day = 86_400
const nsec_per_sec = 1_000_000_000
const nsec_per_day = nsec_per_sec * secs_per_day
const epoch_days = 2_440_587
const epoch_secs = secs_per_day / 2

func jd(t time.Time) (day, nsec int64) {
	sec := t.Unix()
	// guaranteed not to overflow
	day, sec = sec/secs_per_day+epoch_days, sec%secs_per_day+epoch_secs
	return day, sec*nsec_per_sec + int64(t.Nanosecond())
}

// Date returns the Julian day number for t,
// and the nanosecond offset within that day,
// in the range [0, 86399999999999].
func Date(t time.Time) (day, nsec int64) {
	day, nsec = jd(t)
	switch {
	case nsec < 0:
		day -= 1
		nsec += nsec_per_day
	case nsec >= nsec_per_day:
		day += 1
		nsec -= nsec_per_day
	}
	return day, nsec
}

// Float returns the Julian date for t as a float64.
//
// In the XXI century, this has submillisecond precision.
func Float(t time.Time) float64 {
	day, nsec := jd(t)
	// converting day and nsec to float64 is exact
	return float64(day) + float64(nsec)/nsec_per_day
}

// Format returns the Julian date for t as a string.
//
// This has nanosecond precision.
func Format(t time.Time) string {
	var buf [32]byte
	return string(AppendFormat(buf[:0], t))
}

// AppendFormat is like Format but appends the textual representation to dst
// and returns the extended buffer.
func AppendFormat(dst []byte, t time.Time) []byte {
	day, nsec := Date(t)
	if day < 0 && nsec != 0 {
		dst = append(dst, '-')
		day = ^day
		nsec = nsec_per_day - nsec
	}
	var buf [20]byte
	dst = strconv.AppendInt(dst, day, 10)
	frac := strconv.AppendFloat(buf[:0], float64(nsec)/nsec_per_day, 'f', 15, 64)
	return append(dst, bytes.TrimRight(frac[1:], ".0")...)
}

// Time returns the UTC Time corresponding to the Julian day number
// and nanosecond offset within that day.
// Not all day values have a corresponding time value.
func Time(day, nsec int64) time.Time {
	return time.Unix((day-epoch_days)*secs_per_day-epoch_secs, nsec).UTC()
}

// FloatTime returns the UTC Time corresponding to a Julian date.
// Not all date values have a corresponding time value.
//
// In the XXI century, this has submillisecond precision.
func FloatTime(date float64) time.Time {
	day, frac := math.Modf(date)
	nsec := math.Floor(frac * nsec_per_day)
	return Time(int64(day), int64(nsec))
}

// Parse parses a formatted Julian date and returns the UTC Time it represents.
//
// This has nanosecond precision.
func Parse(s string) (time.Time, error) {
	digits := 0
	dot := len(s)
	for i, b := range []byte(s) {
		if '0' <= b && b <= '9' {
			digits++
			continue
		}
		if b == '.' && i < dot {
			dot = i
			continue
		}
		if (b == '+' || b == '-') && i == 0 {
			continue
		}
		return time.Time{}, errors.New("julianday: invalid syntax")
	}
	if digits == 0 {
		return time.Time{}, errors.New("julianday: invalid syntax")
	}

	day, err := strconv.ParseInt(s[:dot], 10, 64)
	if err != nil && dot > 0 {
		return time.Time{}, errors.New("julianday: value out of range")
	}
	frac, _ := strconv.ParseFloat(s[dot:], 64)
	nsec := int64(math.Round(frac * nsec_per_day))
	if s[0] == '-' {
		nsec = -nsec
	}
	return Time(day, nsec), nil
}
