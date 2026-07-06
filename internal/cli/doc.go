// Package cli implements the command-line interface for cal-slotter.
//
// It owns flag parsing, stream-oriented input/output, command dispatch, and the
// orchestration between configuration, Google Calendar access, and slot
// selection. Calendar API details and interval arithmetic stay in their
// dedicated packages so command behavior can be read without following HTTP or
// time-calculation internals.
package cli
