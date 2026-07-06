package slotter

import (
	"testing"
	"time"
)

func TestMerge(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	at := func(h, m int) time.Time {
		return time.Date(2026, 7, 7, h, m, 0, 0, loc)
	}

	got := Merge([]Interval{
		{Start: at(11, 0), End: at(12, 0)},
		{Start: at(10, 30), End: at(11, 30)},
		{Start: at(13, 0), End: at(14, 0)},
	})

	want := []Interval{
		{Start: at(10, 30), End: at(12, 0)},
		{Start: at(13, 0), End: at(14, 0)},
	}
	assertIntervals(t, got, want)
}

func TestAvailableIntervals(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	at := func(h, m int) time.Time {
		return time.Date(2026, 7, 7, h, m, 0, 0, loc)
	}

	got := AvailableIntervals(
		[]Interval{{Start: at(10, 0), End: at(13, 0)}},
		[]Interval{
			{Start: at(9, 0), End: at(10, 30)},
			{Start: at(11, 0), End: at(12, 0)},
			{Start: at(12, 30), End: at(14, 0)},
		},
	)

	want := []Interval{
		{Start: at(10, 30), End: at(11, 0)},
		{Start: at(12, 0), End: at(12, 30)},
	}
	assertIntervals(t, got, want)
}

func TestCandidateSlots(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	at := func(h, m int) time.Time {
		return time.Date(2026, 7, 7, h, m, 0, 0, loc)
	}

	got := CandidateSlots(
		[]Interval{{Start: at(10, 0), End: at(13, 0)}},
		[]Interval{{Start: at(11, 0), End: at(12, 0)}},
		time.Hour,
		30*time.Minute,
		0,
	)

	want := []Interval{
		{Start: at(10, 0), End: at(11, 0)},
		{Start: at(12, 0), End: at(13, 0)},
	}
	assertIntervals(t, got, want)
}

func TestCandidateSlotsRespectsCount(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	at := func(h, m int) time.Time {
		return time.Date(2026, 7, 7, h, m, 0, 0, loc)
	}

	got := CandidateSlots(
		[]Interval{{Start: at(10, 0), End: at(13, 0)}},
		nil,
		time.Hour,
		30*time.Minute,
		2,
	)

	want := []Interval{
		{Start: at(10, 0), End: at(11, 0)},
		{Start: at(10, 30), End: at(11, 30)},
	}
	assertIntervals(t, got, want)
}

func TestSelectSlotsEarlySkipsOverlappingCandidates(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	at := func(h, m int) time.Time {
		return time.Date(2026, 7, 7, h, m, 0, 0, loc)
	}

	got := SelectSlots(
		[]Interval{
			{Start: at(10, 0), End: at(11, 0)},
			{Start: at(10, 30), End: at(11, 30)},
			{Start: at(11, 0), End: at(12, 0)},
		},
		2,
		SelectionEarly,
		loc,
	)

	want := []Interval{
		{Start: at(10, 0), End: at(11, 0)},
		{Start: at(11, 0), End: at(12, 0)},
	}
	assertIntervals(t, got, want)
}

func TestSelectSlotsBalancedSpreadsAcrossDaysAndPeriods(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	at := func(day, h, m int) time.Time {
		return time.Date(2026, 7, day, h, m, 0, 0, loc)
	}

	got := SelectSlots(
		[]Interval{
			{Start: at(7, 10, 0), End: at(7, 11, 0)},
			{Start: at(7, 10, 30), End: at(7, 11, 30)},
			{Start: at(7, 13, 0), End: at(7, 14, 0)},
			{Start: at(8, 10, 0), End: at(8, 11, 0)},
			{Start: at(8, 13, 0), End: at(8, 14, 0)},
			{Start: at(9, 10, 0), End: at(9, 11, 0)},
		},
		5,
		SelectionBalanced,
		loc,
	)

	want := []Interval{
		{Start: at(7, 10, 0), End: at(7, 11, 0)},
		{Start: at(7, 13, 0), End: at(7, 14, 0)},
		{Start: at(8, 10, 0), End: at(8, 11, 0)},
		{Start: at(8, 13, 0), End: at(8, 14, 0)},
		{Start: at(9, 10, 0), End: at(9, 11, 0)},
	}
	assertIntervals(t, got, want)
}

func TestSelectSlotsBalancedChoosesCenterOfPeriod(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	at := func(h, m int) time.Time {
		return time.Date(2026, 7, 7, h, m, 0, 0, loc)
	}

	got := SelectSlots(
		[]Interval{
			{Start: at(13, 0), End: at(14, 0)},
			{Start: at(14, 0), End: at(15, 0)},
			{Start: at(15, 0), End: at(16, 0)},
			{Start: at(16, 0), End: at(17, 0)},
		},
		1,
		SelectionBalanced,
		loc,
	)

	want := []Interval{
		{Start: at(14, 0), End: at(15, 0)},
	}
	assertIntervals(t, got, want)
}

func assertIntervals(t *testing.T, got, want []Interval) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if !got[i].Start.Equal(want[i].Start) || !got[i].End.Equal(want[i].End) {
			t.Fatalf("got[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}
