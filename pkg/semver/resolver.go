package semver

import (
	"fmt"
	"sort"

	masterminds "github.com/Masterminds/semver/v3"
)

const resolverLogPrefix = "semver:resolver"

// VersionRecord represents a version row from the database with a computed string.
type VersionRecord struct {
	ID            string
	Major         int
	Minor         int
	Patch         int
	Prerelease    string
	Status        string // "active", "deprecated", "disabled"
	VersionString string
}

// ToVersionString converts version components to a version string.
func ToVersionString(major, minor, patch int, prerelease string) string {
	base := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	if prerelease != "" {
		return base + "-" + prerelease
	}
	return base
}

// ToVersionRecords converts raw version data into VersionRecord slices with computed strings.
func ToVersionRecords(versions []VersionRecord) []VersionRecord {
	for i := range versions {
		if versions[i].VersionString == "" {
			versions[i].VersionString = ToVersionString(
				versions[i].Major,
				versions[i].Minor,
				versions[i].Patch,
				versions[i].Prerelease,
			)
		}
	}
	return versions
}

// ResolveVersionParams holds parameters for ResolveVersion.
type ResolveVersionParams struct {
	Versions          []VersionRecord
	Range             string // SemVer range, major-only, or empty
	DefaultMajor      int    // -1 means no default
	IncludeDeprecated bool
	ExcludeDisabled   bool
}

// ResolveVersion finds the best matching version for a given range.
func ResolveVersion(params ResolveVersionParams) *VersionRecord {
	// Filter out disabled by default
	filtered := make([]VersionRecord, 0, len(params.Versions))
	for _, v := range params.Versions {
		if params.ExcludeDisabled && v.Status == "disabled" {
			continue
		}
		filtered = append(filtered, v)
	}

	if len(filtered) == 0 {
		return nil
	}

	// Case 1: No range specified - use default major, then latest in that major
	if params.Range == "" {
		if params.DefaultMajor < 0 {
			// No default, pick highest major's latest version
			highestMajor := findHighestMajor(filtered)
			return findLatestInMajor(filtered, highestMajor, params.IncludeDeprecated)
		}
		return findLatestInMajor(filtered, params.DefaultMajor, params.IncludeDeprecated)
	}

	// Case 2: Major-only range (e.g., "3")
	if IsMajorOnly(params.Range) {
		major := ExtractMajorFromRange(params.Range)
		return findLatestInMajor(filtered, major, params.IncludeDeprecated)
	}

	// Case 3: SemVer range (e.g., "^3.2.0", "~3.2.0", ">=3.0.0 <4.0.0")
	constraint, err := masterminds.NewConstraint(params.Range)
	if err != nil {
		// If range parsing fails, try as exact version
		return findExactVersion(filtered, params.Range)
	}

	var matching []VersionRecord
	for _, v := range filtered {
		sv, err := masterminds.NewVersion(v.VersionString)
		if err != nil {
			continue
		}
		if constraint.Check(sv) {
			matching = append(matching, v)
		}
	}

	if len(matching) == 0 {
		return nil
	}

	// Sort by semver descending and pick highest
	sortVersionsDesc(matching)

	// Prefer active over deprecated
	if !params.IncludeDeprecated {
		for i := range matching {
			if matching[i].Status == "active" {
				return &matching[i]
			}
		}
	}

	return &matching[0]
}

// GetUniqueMajors returns all unique major versions sorted descending.
func GetUniqueMajors(versions []VersionRecord) []int {
	seen := make(map[int]bool)
	var majors []int

	for _, v := range versions {
		if !seen[v.Major] {
			seen[v.Major] = true
			majors = append(majors, v.Major)
		}
	}

	sort.Sort(sort.Reverse(sort.IntSlice(majors)))
	return majors
}

// SatisfiesRange checks if a version string satisfies a range.
func SatisfiesRange(version, rangeStr string) bool {
	if IsMajorOnly(rangeStr) {
		sv, err := masterminds.NewVersion(version)
		if err != nil {
			return false
		}
		return int(sv.Major()) == ExtractMajorFromRange(rangeStr)
	}

	constraint, err := masterminds.NewConstraint(rangeStr)
	if err != nil {
		return false
	}

	sv, err := masterminds.NewVersion(version)
	if err != nil {
		return false
	}

	return constraint.Check(sv)
}

// --- internal helpers ---

func findHighestMajor(versions []VersionRecord) int {
	highest := -1
	for _, v := range versions {
		if v.Major > highest {
			highest = v.Major
		}
	}
	return highest
}

func findLatestInMajor(versions []VersionRecord, major int, includeDeprecated bool) *VersionRecord {
	var inMajor []VersionRecord
	for _, v := range versions {
		if v.Major == major {
			inMajor = append(inMajor, v)
		}
	}

	if len(inMajor) == 0 {
		return nil
	}

	// Prefer latest stable (non-prerelease) in major; if none, use latest including prerelease
	var stable []VersionRecord
	for _, v := range inMajor {
		if v.Prerelease == "" {
			stable = append(stable, v)
		}
	}
	candidates := inMajor
	if len(stable) > 0 {
		candidates = stable
	}

	// Sort by minor, patch descending
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Minor != b.Minor {
			return a.Minor > b.Minor
		}
		if a.Patch != b.Patch {
			return a.Patch > b.Patch
		}
		return false
	})

	// Prefer active over deprecated
	if !includeDeprecated {
		for i := range candidates {
			if candidates[i].Status == "active" {
				return &candidates[i]
			}
		}
	}

	return &candidates[0]
}

func findExactVersion(versions []VersionRecord, versionStr string) *VersionRecord {
	for i := range versions {
		if versions[i].VersionString == versionStr {
			return &versions[i]
		}
	}
	return nil
}

func sortVersionsDesc(versions []VersionRecord) {
	sort.Slice(versions, func(i, j int) bool {
		vi, err1 := masterminds.NewVersion(versions[i].VersionString)
		vj, err2 := masterminds.NewVersion(versions[j].VersionString)
		if err1 != nil || err2 != nil {
			return false
		}
		return vi.GreaterThan(vj)
	})
}
