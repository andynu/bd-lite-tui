// Package timegroup maps a timestamp to the relative group it belongs to, so
// the time-based views can render "Today", "Yesterday", "This week" headers
// instead of raw dates.
//
// The mapping is a pure function of the timestamp and a caller-supplied `now`.
// Nothing here reads the clock, so a test can pin the boundary it wants to
// exercise rather than arranging for the machine to be near midnight.
//
// Day and week boundaries are taken in `now`'s location, which is the local
// zone for the running TUI. bd-lite writes RFC 3339 timestamps carrying their
// own offset, so a timestamp written in another zone is compared as the instant
// it names, against boundaries drawn where the reader is sitting.
package timegroup

import "time"

// Bucket is a relative time group. The zero value is Today, and the ordinals
// ascend from newest to oldest, so sorting buckets by their numeric value
// orders group headers newest-first. Sorting by Label() would not: it would put
// "Last week" ahead of "Today".
type Bucket int

const (
	// Today is now's calendar day. Timestamps after now land here too; see Of.
	Today Bucket = iota
	// Yesterday is the calendar day before now's.
	Yesterday
	// ThisWeek is earlier in now's week, before yesterday.
	ThisWeek
	// LastWeek is the seven days before now's week began.
	LastWeek
	// ThisMonth is earlier in now's calendar month, before last week.
	ThisMonth
	// Older is everything before that.
	Older
	// Undated is where a zero timestamp lands. bd-lite's JSONL can omit a
	// time field, which parses as the zero time rather than an error, and a
	// row with no date still has to be rendered somewhere.
	Undated
)

// labels are indexed by Bucket. Keep in sync with the constants above.
var labels = [...]string{
	Today:     "Today",
	Yesterday: "Yesterday",
	ThisWeek:  "This week",
	LastWeek:  "Last week",
	ThisMonth: "This month",
	Older:     "Older",
	Undated:   "Undated",
}

// All returns every bucket in newest-first order. Callers render group headers
// by walking this and skipping the buckets that came out empty.
func All() []Bucket {
	all := make([]Bucket, len(labels))
	for i := range labels {
		all[i] = Bucket(i)
	}
	return all
}

// Label is the group header text for the bucket.
func (b Bucket) Label() string {
	if b < 0 || int(b) >= len(labels) {
		return "Unknown"
	}
	return labels[b]
}

// String implements fmt.Stringer with the header text.
func (b Bucket) String() string { return b.Label() }

// Of returns the bucket t belongs to, relative to now.
//
// A zero t is Undated. A t after now is Today: a timestamp ahead of the clock
// is far more often skew between two machines sharing the JSONL than a genuine
// future date, and Today is the newest bucket either way.
func Of(t, now time.Time) Bucket {
	if t.IsZero() {
		return Undated
	}

	today := startOfDay(now)
	if !t.Before(today) {
		return Today
	}
	if !t.Before(today.AddDate(0, 0, -1)) {
		return Yesterday
	}

	week := startOfWeek(today)
	if !t.Before(week) {
		return ThisWeek
	}
	if !t.Before(week.AddDate(0, 0, -7)) {
		return LastWeek
	}

	if !t.Before(startOfMonth(now)) {
		return ThisMonth
	}
	return Older
}

// startOfDay is midnight opening the calendar day t falls on, in t's location.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// startOfWeek is midnight opening the Monday of dayStart's week. Weeks start on
// Monday to match ISO 8601, so "this week" and "last week" mean the same spans
// the user's other week-numbered tooling means.
func startOfWeek(dayStart time.Time) time.Time {
	offset := (int(dayStart.Weekday()) + 6) % 7 // Monday 0 ... Sunday 6
	return dayStart.AddDate(0, 0, -offset)
}

// startOfMonth is midnight on the first of t's calendar month, in t's location.
func startOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}
