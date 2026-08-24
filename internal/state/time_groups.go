package state

import (
	"sort"
	"time"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/timegroup"
)

// TimeGroup is one rendered group of the time-keyed views: a relative bucket
// and the issues that fall in it, newest first.
type TimeGroup struct {
	Bucket timegroup.Bucket
	Issues []*parser.Issue
}

// TimeKey reads the timestamp a time-keyed view groups on. Created view passes
// CreatedAt; Updated view passes UpdatedAt.
type TimeKey func(*parser.Issue) time.Time

// CreatedAt keys the Created view.
func CreatedAt(issue *parser.Issue) time.Time { return issue.CreatedAt }

// UpdatedAt keys the Updated view. bd-lite bumps this on every write, so an
// issue changes bucket while the TUI is open; the grouping is rebuilt from
// scratch on each repopulate rather than patched in place.
func UpdatedAt(issue *parser.Issue) time.Time { return issue.UpdatedAt }

// GroupByTime buckets the currently visible issues by the timestamp key reads,
// relative to now.
//
// Visible means the same set the status-grouped list view shows: in-progress,
// ready and blocked, plus closed when includeClosed, all after the active
// filters. Only the grouping differs between the two views.
//
// Groups come back in timegroup.All order, which is newest bucket first, with
// empty buckets omitted rather than rendered as a bare header. Within a group
// issues are sorted by key descending, ties broken by ID so a rebuild does not
// shuffle rows under the cursor.
func (s *State) GroupByTime(key TimeKey, now time.Time, includeClosed bool) []TimeGroup {
	visible := make([]*parser.Issue, 0, len(s.issues))
	visible = append(visible, s.GetInProgressIssues()...)
	visible = append(visible, s.GetReadyIssues()...)
	visible = append(visible, s.GetBlockedIssues()...)
	if includeClosed {
		visible = append(visible, s.GetClosedIssues()...)
	}

	byBucket := make(map[timegroup.Bucket][]*parser.Issue, len(timegroup.All()))
	for _, issue := range visible {
		bucket := timegroup.Of(key(issue), now)
		byBucket[bucket] = append(byBucket[bucket], issue)
	}

	groups := make([]TimeGroup, 0, len(byBucket))
	for _, bucket := range timegroup.All() {
		issues := byBucket[bucket]
		if len(issues) == 0 {
			continue
		}
		sort.SliceStable(issues, func(i, j int) bool {
			a, b := key(issues[i]), key(issues[j])
			if a.Equal(b) {
				return issues[i].ID < issues[j].ID
			}
			return a.After(b)
		})
		groups = append(groups, TimeGroup{Bucket: bucket, Issues: issues})
	}
	return groups
}
