package timefmt

import (
	"testing"
	"time"

	"github.com/kokoichi206/cal-slotter/internal/slotter"
)

func TestParseWindow(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	got, err := ParseWindow("2026-07-07 10:00-18:00", loc)
	if err != nil {
		t.Fatal(err)
	}

	want := slotter.Interval{
		Start: time.Date(2026, 7, 7, 10, 0, 0, 0, loc),
		End:   time.Date(2026, 7, 7, 18, 0, 0, 0, loc),
	}
	if !got.Start.Equal(want.Start) || !got.End.Equal(want.End) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestFormatCustomerSlot(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	slot := slotter.Interval{
		Start: time.Date(2026, 7, 7, 14, 0, 0, 0, loc),
		End:   time.Date(2026, 7, 7, 15, 0, 0, 0, loc),
	}

	got := FormatCustomerSlot(slot)
	want := "7/7(火) 14:00〜15:00"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
