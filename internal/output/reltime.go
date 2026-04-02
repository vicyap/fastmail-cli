package output

import (
	"fmt"
	"time"
)

// RelativeTime formats a time as a human-readable relative string.
// For times within the last 24 hours, returns "Xm ago", "Xh ago".
// For times within the last 7 days, returns "Mon", "Tue", etc.
// For times within the current year, returns "Jan 02".
// Otherwise returns "2006-01-02".
func RelativeTime(t time.Time) string {
	return relativeTimeFrom(t, time.Now())
}

func relativeTimeFrom(t time.Time, now time.Time) string {
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return t.Format("Mon 15:04")
	case t.Year() == now.Year():
		return t.Format("Jan 02")
	default:
		return t.Format("2006-01-02")
	}
}
