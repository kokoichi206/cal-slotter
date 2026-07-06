// Package googlecal contains the Google Calendar boundary for cal-slotter.
//
// It handles OAuth token creation, authenticated HTTP requests, freebusy reads,
// temporary hold creation, and hold cleanup. The package returns domain-level
// intervals and errors to callers; it does not decide which candidate slots are
// good or format text for customers.
package googlecal
