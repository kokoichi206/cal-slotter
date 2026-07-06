// Package config defines and loads local cal-slotter configuration.
//
// Configuration is intentionally small: timezone, target calendar, OAuth file
// paths, and default member calendars. The package only deals with local JSON
// files and path defaults; command-line overrides are applied by internal/cli.
package config
