package timefmt

import (
	"fmt"
	"strings"
	"time"

	"github.com/kokoichi206/cal-slotter/internal/slotter"
)

var japaneseWeekdays = [...]string{"日", "月", "火", "水", "木", "金", "土"}

// FormatCustomerSlot formats a slot as Japanese customer-facing text.
func FormatCustomerSlot(slot slotter.Interval) string {
	weekday := japaneseWeekdays[int(slot.Start.Weekday())]
	return fmt.Sprintf("%d/%d(%s) %s〜%s",
		int(slot.Start.Month()),
		slot.Start.Day(),
		weekday,
		slot.Start.Format("15:04"),
		slot.End.Format("15:04"),
	)
}

// ParseWindow parses a CLI window in "YYYY-MM-DD HH:MM-HH:MM" format.
func ParseWindow(value string, loc *time.Location) (slotter.Interval, error) {
	date, startText, endText, err := splitDateTimeRange(value)
	if err != nil {
		return slotter.Interval{}, err
	}

	start, err := time.ParseInLocation("2006-01-02 15:04", date+" "+startText, loc)
	if err != nil {
		return slotter.Interval{}, fmt.Errorf("parse start time %q: %w", value, err)
	}
	end, err := time.ParseInLocation("2006-01-02 15:04", date+" "+endText, loc)
	if err != nil {
		return slotter.Interval{}, fmt.Errorf("parse end time %q: %w", value, err)
	}
	if !start.Before(end) {
		return slotter.Interval{}, fmt.Errorf("time range start must be before end: %q", value)
	}

	return slotter.Interval{Start: start, End: end}, nil
}

// ParseKeep parses a confirm keep time in "YYYY-MM-DD HH:MM" format.
func ParseKeep(value string, loc *time.Location) (time.Time, error) {
	keep, err := time.ParseInLocation("2006-01-02 15:04", value, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse keep time %q: %w", value, err)
	}
	return keep, nil
}

func splitDateTimeRange(value string) (string, string, string, error) {
	fields := strings.Fields(value)
	if len(fields) != 2 {
		return "", "", "", fmt.Errorf("expected %q format, got %q", "YYYY-MM-DD HH:MM-HH:MM", value)
	}
	timeRange := strings.Split(fields[1], "-")
	if len(timeRange) != 2 {
		return "", "", "", fmt.Errorf("expected %q format, got %q", "YYYY-MM-DD HH:MM-HH:MM", value)
	}
	return fields[0], timeRange[0], timeRange[1], nil
}
