package goversion

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
)

var (
	versionRegexp    *regexp.Regexp
	constraintRegexp *regexp.Regexp
)

var initRegexpOnce sync.Once

func initRegexp() {
	initRegexpOnce.Do(func() {
		constraintRegexp = regexp.MustCompile(`(\A|[\s|])([><=~^][ ><=~^]*)?(x|X|\*|\d+)(?:\.(x|X|\*|\d+))?(?:\.(x|X|\*|\d+))?([[:alpha:]])?`)
		versionRegexp = regexp.MustCompile(`^go(\d+)(?:\.(\d+))?(?:\.(\d+))?([[:alnum:]]+)?$`)
	})
}

// ErrInvalidGoVersion is returned when a go version is not valid
var ErrInvalidGoVersion = fmt.Errorf("invalid go version")

// ErrInvalidConstraint is returned when a constraint is not valid
var ErrInvalidConstraint = fmt.Errorf("invalid go constraint")

// go2semverString returns the semver equivalent of version
func go2semverString(version string) string {
	initRegexp()
	if !strings.HasPrefix(version, "go") {
		version = "go" + version
	}
	parts := versionRegexp.FindStringSubmatch(version)
	if len(parts) == 0 {
		return ""
	}
	for i := 1; i < 4; i++ {
		if parts[i] == "" {
			parts[i] = "0"
		}
	}
	sv := fmt.Sprintf("%s.%s.%s", parts[1], parts[2], parts[3])
	if parts[4] != "" {
		sv += "-" + parts[4]
	}
	return sv
}

// Version represents a single go version
type Version struct {
	semver   *semver.Version
	original string
}

// NewVersion parses a given version and returns an instance of Version or
// an error if unable to parse the version.
func NewVersion(version string) (*Version, error) {
	sv, err := semver.StrictNewVersion(go2semverString(version))
	if err != nil {
		return nil, ErrInvalidGoVersion
	}
	return &Version{
		semver:   sv,
		original: version,
	}, nil
}

// LessThan tests if v is less than o.
func (v *Version) LessThan(o *Version) bool {
	return v.semver.LessThan(o.semver)
}

// GreaterThan tests if v is less than o.
func (v *Version) GreaterThan(o *Version) bool {
	return v.semver.GreaterThan(o.semver)
}

// Equal tests if v is equal to o.
func (v *Version) Equal(o *Version) bool {
	return v.semver.Equal(o.semver)
}

// IsStable returns true if the version is stable meaning it has no prerelease
func (v *Version) IsStable() bool {
	return v.semver.Prerelease() == ""
}

// String returns the string representation of this version.
func (v Version) String() string {
	sv := v.semver
	if sv.Patch() != 0 {
		return fmt.Sprintf("go%d.%d.%d%s", sv.Major(), sv.Minor(), sv.Patch(), sv.Prerelease())
	}
	if sv.Minor() != 0 {
		return fmt.Sprintf("go%d.%d%s", sv.Major(), sv.Minor(), sv.Prerelease())
	}
	return fmt.Sprintf("go%d%s", sv.Major(), sv.Prerelease())
}

func go2SemverRange(goRange string) string {
	initRegexp()
	return constraintRegexp.ReplaceAllStringFunc(goRange, func(s string) string {
		sm := constraintRegexp.FindStringSubmatch(s)
		if len(sm) < 7 {
			return sm[0]
		}
		stopZeros := strings.ContainsAny(sm[3], `Xx*`)
		if !stopZeros {
			if sm[4] == "" {
				sm[4] = "0"
			}
			stopZeros = strings.ContainsAny(sm[4], `Xx*`)
		}
		if !stopZeros {
			if sm[5] == "" {
				sm[5] = "0"
			}
		}
		if sm[4] != "" {
			sm[4] = "." + sm[4]
		}
		if sm[5] != "" {
			sm[5] = "." + sm[5]
		}
		result := strings.Join(sm[1:6], "")
		if sm[6] != "" {
			result += "-" + sm[6]
		}
		return result
	})
}

// NewConstraints returns a Constraints instance that a Version instance can
// be checked against.
func NewConstraints(c string) (*Constraints, error) {
	semverRange := go2SemverRange(c)
	constraints, err := semver.NewConstraint(semverRange)
	if err != nil {
		return nil, ErrInvalidConstraint
	}
	return &Constraints{
		constraints: constraints,
	}, nil
}

// Constraints is one of more constraint that a go version can be checked against.
type Constraints struct {
	constraints *semver.Constraints
}

// Check tests if v satisfies the constraints.
func (c Constraints) Check(v *Version) bool {
	return c.constraints.Check(v.semver)
}

// FilterVersions returns a slice of Versions that satisfy Constraints
func (c Constraints) FilterVersions(versions []*Version) []*Version {
	result := make([]*Version, 0, len(versions))
	for _, version := range versions {
		if c.Check(version) {
			result = append(result, version)
		}
	}
	return result
}
