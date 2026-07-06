package slotter

import (
	"slices"
	"time"
)

type Interval struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type SelectionStrategy string

const (
	SelectionBalanced SelectionStrategy = "balanced"
	SelectionEarly    SelectionStrategy = "early"
)

func (i Interval) Duration() time.Duration {
	return i.End.Sub(i.Start)
}

func (i Interval) In(loc *time.Location) Interval {
	return Interval{
		Start: i.Start.In(loc),
		End:   i.End.In(loc),
	}
}

func Merge(intervals []Interval) []Interval {
	if len(intervals) == 0 {
		return nil
	}

	merged := slices.Clone(intervals)
	slices.SortFunc(merged, func(a, b Interval) int {
		return a.Start.Compare(b.Start)
	})

	out := []Interval{merged[0]}
	for _, current := range merged[1:] {
		last := &out[len(out)-1]
		if !current.Start.After(last.End) {
			if current.End.After(last.End) {
				last.End = current.End
			}
			continue
		}
		out = append(out, current)
	}

	return out
}

func AvailableIntervals(windows, busy []Interval) []Interval {
	mergedBusy := Merge(busy)
	var out []Interval

	for _, window := range windows {
		cursor := window.Start
		for _, busyInterval := range mergedBusy {
			if !busyInterval.End.After(window.Start) {
				continue
			}
			if !busyInterval.Start.Before(window.End) {
				break
			}
			if busyInterval.Start.After(cursor) {
				out = append(out, Interval{Start: cursor, End: minTime(busyInterval.Start, window.End)})
			}
			if busyInterval.End.After(cursor) {
				cursor = maxTime(busyInterval.End, cursor)
			}
			if !cursor.Before(window.End) {
				break
			}
		}
		if cursor.Before(window.End) {
			out = append(out, Interval{Start: cursor, End: window.End})
		}
	}

	return out
}

func CandidateSlots(windows, busy []Interval, duration, step time.Duration, count int) []Interval {
	mergedBusy := Merge(busy)
	var slots []Interval

	for _, window := range windows {
		for start := window.Start; !start.Add(duration).After(window.End); start = start.Add(step) {
			slot := Interval{Start: start, End: start.Add(duration)}
			if overlapsAny(slot, mergedBusy) {
				continue
			}
			slots = append(slots, slot)
			if count > 0 && len(slots) == count {
				return slots
			}
		}
	}

	return slots
}

func SelectSlots(slots []Interval, count int, strategy SelectionStrategy, loc *time.Location) []Interval {
	switch strategy {
	case SelectionBalanced:
		return selectBalanced(slots, count, loc)
	case SelectionEarly:
		return selectEarly(slots, count)
	default:
		return nil
	}
}

func selectEarly(slots []Interval, count int) []Interval {
	var selected []Interval
	for _, slot := range slots {
		if overlapsAny(slot, selected) {
			continue
		}
		selected = append(selected, slot)
		if count > 0 && len(selected) == count {
			return selected
		}
	}
	return selected
}

func selectBalanced(slots []Interval, count int, loc *time.Location) []Interval {
	type bucketKey struct {
		day    string
		period string
	}

	var keys []bucketKey
	buckets := make(map[bucketKey][]Interval)
	seen := make(map[bucketKey]bool)

	for _, slot := range slots {
		key := bucketKey{
			day:    slot.Start.In(loc).Format("2006-01-02"),
			period: dayPeriod(slot.Start.In(loc)),
		}
		if !seen[key] {
			keys = append(keys, key)
			seen[key] = true
		}
		buckets[key] = append(buckets[key], slot)
	}

	var selected []Interval
	selectedByBucket := make(map[bucketKey][]Interval)
	for {
		progress := false
		for _, key := range keys {
			slot, remaining, ok := takeSpreadSlot(buckets[key], selected, selectedByBucket[key])
			if !ok {
				buckets[key] = remaining
				continue
			}
			buckets[key] = remaining
			selected = append(selected, slot)
			selectedByBucket[key] = append(selectedByBucket[key], slot)
			progress = true
			if count > 0 && len(selected) == count {
				return sortIntervals(selected)
			}
		}
		if !progress {
			return sortIntervals(selected)
		}
	}
}

func takeSpreadSlot(slots, selected, selectedInBucket []Interval) (Interval, []Interval, bool) {
	candidates := make([]Interval, 0, len(slots))
	for _, slot := range slots {
		if !overlapsAny(slot, selected) {
			candidates = append(candidates, slot)
		}
	}
	if len(candidates) == 0 {
		return Interval{}, nil, false
	}

	bestIndex := 0
	bestScore := spreadScore(candidates[0], candidates, selectedInBucket)
	for i, candidate := range candidates[1:] {
		score := spreadScore(candidate, candidates, selectedInBucket)
		if score > bestScore {
			bestIndex = i + 1
			bestScore = score
		}
	}

	selectedSlot := candidates[bestIndex]
	remaining := make([]Interval, 0, len(candidates)-1)
	for i, candidate := range candidates {
		if i != bestIndex {
			remaining = append(remaining, candidate)
		}
	}
	return selectedSlot, remaining, true
}

func spreadScore(slot Interval, bucket, selected []Interval) int64 {
	centerDistance := -absDuration(slot.Start.Sub(bucketCenter(bucket)))
	if len(selected) == 0 {
		return centerDistance
	}
	return minStartDistance(slot, selected)*1000 + centerDistance
}

func bucketCenter(slots []Interval) time.Time {
	minStart := slots[0].Start
	maxStart := slots[0].Start
	for _, slot := range slots[1:] {
		if slot.Start.Before(minStart) {
			minStart = slot.Start
		}
		if slot.Start.After(maxStart) {
			maxStart = slot.Start
		}
	}
	return minStart.Add(maxStart.Sub(minStart) / 2)
}

func minStartDistance(slot Interval, selected []Interval) int64 {
	minDistance := absDuration(slot.Start.Sub(selected[0].Start))
	for _, selectedSlot := range selected[1:] {
		distance := absDuration(slot.Start.Sub(selectedSlot.Start))
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance
}

func absDuration(duration time.Duration) int64 {
	if duration < 0 {
		return int64(-duration)
	}
	return int64(duration)
}

func sortIntervals(intervals []Interval) []Interval {
	out := slices.Clone(intervals)
	slices.SortFunc(out, func(a, b Interval) int {
		return a.Start.Compare(b.Start)
	})
	return out
}

func dayPeriod(t time.Time) string {
	if t.Hour() < 12 {
		return "morning"
	}
	return "afternoon"
}

func overlapsAny(slot Interval, intervals []Interval) bool {
	for _, interval := range intervals {
		if !interval.End.After(slot.Start) {
			continue
		}
		if !interval.Start.Before(slot.End) {
			return false
		}
		if slot.Start.Before(interval.End) && interval.Start.Before(slot.End) {
			return true
		}
	}
	return false
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
