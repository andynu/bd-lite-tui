package timegroup

import (
	"testing"
	"time"
)

// testZone is a fixed offset rather than time.Local so the boundaries under
// test do not move with the machine running the suite.
var testZone = time.FixedZone("TEST", -5*60*60)

func ts(y int, m time.Month, d, hh, mm, ss int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, 0, testZone)
}

// now is Wednesday 2026-08-19 14:30 in testZone. Mid-week and mid-month, so
// every bucket has a non-empty span:
//
//	Today      Aug 19
//	Yesterday  Aug 18
//	This week  Aug 17          (week opens Monday Aug 17)
//	Last week  Aug 10 .. 16
//	This month Aug  1 ..  9
//	Older      before Aug 1
var now = ts(2026, time.August, 19, 14, 30, 0)

func TestOfBuckets(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want Bucket
	}{
		{"now itself", now, Today},
		{"first instant of today", ts(2026, time.August, 19, 0, 0, 0), Today},
		{"last instant of today", ts(2026, time.August, 19, 23, 59, 59), Today},
		{"later today than now", ts(2026, time.August, 19, 23, 0, 0), Today},
		{"tomorrow, i.e. clock skew", ts(2026, time.August, 20, 9, 0, 0), Today},
		{"far future", ts(2030, time.January, 1, 0, 0, 0), Today},

		{"last instant of yesterday", ts(2026, time.August, 18, 23, 59, 59), Yesterday},
		{"first instant of yesterday", ts(2026, time.August, 18, 0, 0, 0), Yesterday},

		{"last instant before yesterday", ts(2026, time.August, 17, 23, 59, 59), ThisWeek},
		{"monday, first instant of this week", ts(2026, time.August, 17, 0, 0, 0), ThisWeek},

		{"sunday, last instant of last week", ts(2026, time.August, 16, 23, 59, 59), LastWeek},
		{"monday, first instant of last week", ts(2026, time.August, 10, 0, 0, 0), LastWeek},

		{"last instant before last week", ts(2026, time.August, 9, 23, 59, 59), ThisMonth},
		{"first of the month", ts(2026, time.August, 1, 0, 0, 0), ThisMonth},

		{"last instant of the previous month", ts(2026, time.July, 31, 23, 59, 59), Older},
		{"a year back", ts(2025, time.August, 19, 14, 30, 0), Older},

		{"zero time", time.Time{}, Undated},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Of(tc.in, now); got != tc.want {
				t.Errorf("Of(%s, %s) = %v, want %v",
					tc.in.Format(time.RFC3339), now.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

func TestOfUsesLocalDayBoundaries(t *testing.T) {
	// now is late evening local, which is already the next calendar day in
	// UTC. A timestamp from this morning is Today; bucketing on UTC days
	// would call it Yesterday.
	evening := ts(2026, time.August, 19, 20, 0, 0)
	morning := ts(2026, time.August, 19, 1, 0, 0)

	if got := Of(morning, evening); got != Today {
		t.Errorf("Of(local morning, local evening) = %v, want Today (UTC days differ: %s vs %s)",
			got, morning.UTC().Format(time.RFC3339), evening.UTC().Format(time.RFC3339))
	}
}

func TestOfComparesInstantsAcrossZones(t *testing.T) {
	// bd-lite writes each timestamp with its own offset, so a JSONL row
	// committed elsewhere arrives in another zone. It must be bucketed as the
	// instant it names, against boundaries drawn where the reader sits.
	evening := ts(2026, time.August, 19, 20, 0, 0)
	sameMorningInUTC := time.Date(2026, time.August, 19, 6, 0, 0, 0, time.UTC)

	if got := Of(sameMorningInUTC, evening); got != Today {
		t.Errorf("Of(%s, %s) = %v, want Today",
			sameMorningInUTC.Format(time.RFC3339), evening.Format(time.RFC3339), got)
	}
}

func TestOfWhenNowIsMonday(t *testing.T) {
	// The week opens under the caller's feet: on a Monday "this week" is empty
	// and yesterday (Sunday) belongs to the week before.
	monday := ts(2026, time.August, 24, 9, 0, 0)
	if monday.Weekday() != time.Monday {
		t.Fatalf("fixture drifted: %s is a %s", monday.Format("2006-01-02"), monday.Weekday())
	}

	tests := []struct {
		name string
		in   time.Time
		want Bucket
	}{
		{"sunday is yesterday, not last week", ts(2026, time.August, 23, 12, 0, 0), Yesterday},
		{"saturday falls in last week", ts(2026, time.August, 22, 12, 0, 0), LastWeek},
		{"the monday before opens last week", ts(2026, time.August, 17, 0, 0, 0), LastWeek},
		{"the sunday before that is this month", ts(2026, time.August, 16, 23, 59, 59), ThisMonth},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Of(tc.in, monday); got != tc.want {
				t.Errorf("Of(%s, monday) = %v, want %v", tc.in.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

func TestOfWhenNowIsSunday(t *testing.T) {
	// Sunday closes an ISO week, so the whole week back to Monday is "this
	// week" apart from today and yesterday.
	sunday := ts(2026, time.August, 23, 18, 0, 0)
	if sunday.Weekday() != time.Sunday {
		t.Fatalf("fixture drifted: %s is a %s", sunday.Format("2006-01-02"), sunday.Weekday())
	}

	if got := Of(ts(2026, time.August, 17, 8, 0, 0), sunday); got != ThisWeek {
		t.Errorf("Of(monday, sunday) = %v, want ThisWeek", got)
	}
	if got := Of(ts(2026, time.August, 16, 23, 59, 59), sunday); got != LastWeek {
		t.Errorf("Of(previous sunday, sunday) = %v, want LastWeek", got)
	}
}

func TestOfSpansAMonthBoundaryWithinAWeek(t *testing.T) {
	// Thursday Sep 3 2026: this week opens Monday Aug 31, so two days of
	// "this week" sit in the previous calendar month. Week membership is
	// checked before month membership, so they stay in the week bucket
	// instead of being demoted to Older.
	thursday := ts(2026, time.September, 3, 10, 0, 0)
	if thursday.Weekday() != time.Thursday {
		t.Fatalf("fixture drifted: %s is a %s", thursday.Format("2006-01-02"), thursday.Weekday())
	}

	if got := Of(ts(2026, time.August, 31, 9, 0, 0), thursday); got != ThisWeek {
		t.Errorf("Of(Aug 31, Sep 3) = %v, want ThisWeek", got)
	}
	if got := Of(ts(2026, time.August, 24, 9, 0, 0), thursday); got != LastWeek {
		t.Errorf("Of(Aug 24, Sep 3) = %v, want LastWeek", got)
	}
	if got := Of(ts(2026, time.August, 23, 9, 0, 0), thursday); got != Older {
		t.Errorf("Of(Aug 23, Sep 3) = %v, want Older (before last week, and a month back)", got)
	}
}

func TestBucketOrdinalsRunNewestFirst(t *testing.T) {
	// Group headers are ordered by the ordinal, so it has to ascend from the
	// newest bucket to the oldest. Undated sorts last: a row with no date is
	// the least interesting thing on the screen.
	ordered := []Bucket{Today, Yesterday, ThisWeek, LastWeek, ThisMonth, Older, Undated}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1] >= ordered[i] {
			t.Errorf("%v (%d) does not sort before %v (%d)",
				ordered[i-1], int(ordered[i-1]), ordered[i], int(ordered[i]))
		}
	}

	all := All()
	if len(all) != len(ordered) {
		t.Fatalf("All() returned %d buckets, want %d", len(all), len(ordered))
	}
	for i, want := range ordered {
		if all[i] != want {
			t.Errorf("All()[%d] = %v, want %v", i, all[i], want)
		}
	}
}

func TestAllIsNotAliased(t *testing.T) {
	// Renderers walk All() and may sort or trim it; that must not corrupt the
	// next caller's copy.
	first := All()
	first[0] = Older
	if All()[0] != Today {
		t.Error("All() handed out a slice backed by shared state")
	}
}

func TestBucketLabels(t *testing.T) {
	seen := map[string]Bucket{}
	for _, b := range All() {
		label := b.Label()
		if label == "" {
			t.Errorf("Bucket(%d) has an empty label", int(b))
		}
		if prev, dup := seen[label]; dup {
			t.Errorf("Bucket(%d) and Bucket(%d) both label as %q", int(prev), int(b), label)
		}
		seen[label] = b
		if b.String() != label {
			t.Errorf("Bucket(%d).String() = %q, want %q", int(b), b.String(), label)
		}
	}
}

func TestBucketLabelOutOfRange(t *testing.T) {
	// A header must never panic on a bucket value that came from somewhere
	// other than Of.
	for _, b := range []Bucket{-1, Bucket(len(labels)), Bucket(99)} {
		if got := b.Label(); got != "Unknown" {
			t.Errorf("Bucket(%d).Label() = %q, want %q", int(b), got, "Unknown")
		}
	}
}
