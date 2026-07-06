// Package releaseguard validates release tag invariants used by CI.
package releaseguard

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a stable release version parsed from a vMAJOR.MINOR.PATCH tag.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseTag parses a stable release tag.
func ParseTag(tag string) (Version, error) {
	rest, ok := strings.CutPrefix(tag, "v")
	if !ok {
		return Version{}, fmt.Errorf("release tag %q must start with v", tag)
	}

	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("release tag %q must use vMAJOR.MINOR.PATCH", tag)
	}

	major, err := parsePart(tag, parts[0])
	if err != nil {
		return Version{}, err
	}
	minor, err := parsePart(tag, parts[1])
	if err != nil {
		return Version{}, err
	}
	patch, err := parsePart(tag, parts[2])
	if err != nil {
		return Version{}, err
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// Compare returns -1 when v is older than other, 0 when equal, and 1 when newer.
func (v Version) Compare(other Version) int {
	switch {
	case v.Major != other.Major:
		return compareInt(v.Major, other.Major)
	case v.Minor != other.Minor:
		return compareInt(v.Minor, other.Minor)
	default:
		return compareInt(v.Patch, other.Patch)
	}
}

// Validate checks that tag points at the release base and is newer than existing release tags.
func Validate(tag string, tagCommit string, baseCommit string, existingTags []string) error {
	current, err := ParseTag(tag)
	if err != nil {
		return err
	}

	if tagCommit != baseCommit {
		return fmt.Errorf("release tag %s points at %s, but origin/main is %s", tag, tagCommit, baseCommit)
	}

	for _, existingTag := range existingTags {
		if existingTag == tag {
			continue
		}

		existing, err := ParseTag(existingTag)
		if err != nil {
			continue
		}
		if current.Compare(existing) <= 0 {
			return fmt.Errorf("release tag %s must be newer than existing tag %s", tag, existingTag)
		}
	}

	return nil
}

func parsePart(tag string, part string) (int, error) {
	if part == "" {
		return 0, fmt.Errorf("release tag %q must use vMAJOR.MINOR.PATCH", tag)
	}
	if len(part) > 1 && part[0] == '0' {
		return 0, fmt.Errorf("release tag %q must not contain leading zeroes", tag)
	}

	value, err := strconv.Atoi(part)
	if err != nil {
		return 0, fmt.Errorf("release tag %q must use numeric version parts", tag)
	}
	return value, nil
}

func compareInt(a int, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
