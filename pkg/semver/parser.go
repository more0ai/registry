// Package semver provides capability reference parsing and SemVer resolution logic.
package semver

import (
	"fmt"
	"regexp"
	"strings"
)

const logPrefix = "semver:parser"

// ParsedCapabilityRef holds the parsed components of a capability reference string.
type ParsedCapabilityRef struct {
	// Full capability string (e.g., "more0.doc.ingest")
	Full string
	// Application namespace (e.g., "more0")
	App string
	// Capability name within app (e.g., "doc.ingest")
	Name string
	// Version range if specified (e.g., "^3.2.0", "3", ""); empty string means no version
	Range string
	// Raw input string
	Raw string
}

var (
	capabilityNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)
	appNameRegex        = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	majorOnlyRegex      = regexp.MustCompile(`^\d+$`)
	exactVersionRegex   = regexp.MustCompile(`^\d+\.\d+\.\d+(-[\w.]+)?(\+[\w.]+)?$`)
)

// ParseCapabilityRef parses a capability reference string.
//
// Supported formats:
//   - more0.doc.ingest           (no version)
//   - more0.doc.ingest@3         (major only)
//   - more0.doc.ingest@3.2.1     (exact version)
//   - more0.doc.ingest@^3.2.0    (caret range)
//   - more0.doc.ingest@~3.2.0    (tilde range)
//   - more0.doc.ingest@>=3.0.0   (comparison range)
func ParseCapabilityRef(input string) (*ParsedCapabilityRef, error) {
	raw := strings.TrimSpace(input)

	// Split on @ to separate capability from version
	atIndex := strings.Index(raw, "@")

	var capPart string
	var rangeStr string

	if atIndex == -1 {
		capPart = raw
	} else {
		capPart = raw[:atIndex]
		rangeStr = raw[atIndex+1:]
	}

	// Parse capability part: app.name (name can have dots)
	firstDot := strings.Index(capPart, ".")
	if firstDot == -1 {
		return nil, fmt.Errorf("%s - invalid capability format, missing app: %s", logPrefix, raw)
	}

	app := capPart[:firstDot]
	name := capPart[firstDot+1:]

	if app == "" || name == "" {
		return nil, fmt.Errorf("%s - invalid capability format: %s", logPrefix, raw)
	}

	return &ParsedCapabilityRef{
		Full:  capPart,
		App:   app,
		Name:  name,
		Range: rangeStr,
		Raw:   raw,
	}, nil
}

// IsMajorOnly checks if a range is a major-only specifier (e.g., "3").
func IsMajorOnly(rangeStr string) bool {
	return majorOnlyRegex.MatchString(rangeStr)
}

// IsExactVersion checks if a range is an exact version (e.g., "3.2.1").
func IsExactVersion(rangeStr string) bool {
	return exactVersionRegex.MatchString(rangeStr)
}

// ExtractMajorFromRange extracts the major version if the range is major-only.
// Returns -1 if not a major-only range.
func ExtractMajorFromRange(rangeStr string) int {
	if !IsMajorOnly(rangeStr) {
		return -1
	}
	var major int
	fmt.Sscanf(rangeStr, "%d", &major)
	return major
}

// BuildCapabilityString builds a full capability string from parts.
func BuildCapabilityString(params BuildCapabilityParams) string {
	base := params.App + "." + params.Name
	if params.Version != "" {
		return base + "@" + params.Version
	}
	return base
}

// BuildCapabilityParams holds parameters for BuildCapabilityString.
type BuildCapabilityParams struct {
	App     string
	Name    string
	Version string
}

// ValidateCapabilityName validates a capability name (allows letters, digits, dots, hyphens, underscores).
func ValidateCapabilityName(name string) bool {
	return capabilityNameRegex.MatchString(name)
}

// ValidateAppName validates an app name (lowercase, alphanumeric, hyphens).
func ValidateAppName(app string) bool {
	return appNameRegex.MatchString(app)
}
