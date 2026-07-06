package version

import (
	"fmt"
	"runtime"
)

// Version is the release version. It is intended to be set by -ldflags.
var Version = "dev"

// Commit is the source commit. It is intended to be set by -ldflags.
var Commit = "unknown"

// Date is the build or release date. It is intended to be set by -ldflags.
var Date = "unknown"

// Info contains build metadata displayed by the version command.
type Info struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
}

// Current returns the current build metadata.
func Current() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
	}
}

// String formats build metadata for human-facing CLI output.
func (i Info) String() string {
	return fmt.Sprintf("slotter version %s\ncommit: %s\ndate: %s\ngo: %s", i.Version, i.Commit, i.Date, i.GoVersion)
}
