// Package goreleases fetches Go release data from go.dev/dl.
package goreleases

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/willabides/goversions/goversion"
)

// FetchReleasesOptions options for FetchReleases
type FetchReleasesOptions struct {
	HTTPClient   *http.Client
	SkipVersions []string // go versions to skip ( like go1.7.2 which was pulled )
}

// FetchReleases fetches release data from go.dev/dl
func FetchReleases(ctx context.Context, options *FetchReleasesOptions) ([]Release, error) {
	httpClient := http.DefaultClient
	if options == nil {
		options = new(FetchReleasesOptions)
	}
	if options.HTTPClient != nil {
		httpClient = options.HTTPClient
	}
	gc := &gldoClient{
		httpClient: httpClient,
	}
	releases, err := gc.fetchReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching releases: %v", err)
	}
	filtered := make([]Release, 0, len(releases))
	for _, r := range releases {
		if skipVersion(r.Version, options.SkipVersions) {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.Sort(sort.Reverse(releaseSorter(filtered)))
	return filtered, nil
}

func goVersionLess(a, b string) bool {
	var invalidA, invalidB bool
	verA, err := goversion.NewVersion(a)
	if err != nil {
		invalidA = true
	}
	verB, err := goversion.NewVersion(b)
	if err != nil {
		invalidB = true
	}
	if invalidB {
		return false
	}
	if invalidA {
		return true
	}
	return verA.LessThan(verB)
}

// Release is a go release
type Release struct {
	Version string        `json:"version"`
	Stable  bool          `json:"stable"`
	Files   []ReleaseFile `json:"files"`
}

func (f Release) less(other Release) bool {
	return goVersionLess(f.Version, other.Version)
}

type releaseSorter []Release

func (r releaseSorter) Len() int {
	return len(r)
}

func (r releaseSorter) Less(i, j int) bool {
	return r[i].less(r[j])
}

func (r releaseSorter) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// ReleaseFile is a file included in a go release
type ReleaseFile struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Sha256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Kind     string `json:"kind"`
}

func (f ReleaseFile) less(other ReleaseFile) bool {
	if goVersionLess(f.Version, other.Version) {
		return true
	}
	if goVersionLess(other.Version, f.Version) {
		return false
	}
	return f.Filename < other.Filename
}

type releaseFileSorter []ReleaseFile

func (r releaseFileSorter) Len() int {
	return len(r)
}

func (r releaseFileSorter) Less(i, j int) bool {
	return r[i].less(r[j])
}

func (r releaseFileSorter) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func skipVersion(version string, skips []string) bool {
	for _, skip := range skips {
		if skip == version {
			return true
		}
	}
	return false
}

func findReleaseByVersion(releases []Release, version string) (Release, bool) {
	for _, release := range releases {
		if release.Version == version {
			return release, true
		}
	}
	return Release{}, false
}
