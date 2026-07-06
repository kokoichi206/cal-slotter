// Package slotter contains calendar-agnostic interval and slot selection logic.
//
// It receives windows, busy intervals, durations, and selection strategy, then
// returns candidate meeting slots. It has no Google Calendar, config, CLI, or
// presentation concerns, which keeps the scheduling rules unit-testable.
package slotter
