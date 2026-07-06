// Command slotter provides the cal-slotter CLI entry point.
//
// The command keeps main small: it delegates argument handling and business
// flow to internal/cli, then maps returned errors to stderr and process exit
// codes.
package main
