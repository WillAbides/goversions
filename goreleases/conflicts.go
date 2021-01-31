package goreleases

import (
	"fmt"
	"sort"

	"github.com/google/go-cmp/cmp"
)

// FindConflicts returns conflicts that would prevent automatically merging head into base.
// Conflicts include missing releases in head and any change to an existing release.
func FindConflicts(base, head []Release) []string {
	var msgs []string
	baseSeen := map[string]bool{}
	headSeen := map[string]bool{}
	for _, baseRelease := range base {
		if baseRelease.Version == "" {
			msgs = append(msgs, "base has a release with no version")
			continue
		}
		if baseSeen[baseRelease.Version] {
			msgs = append(msgs, fmt.Sprintf("base has multiple releases with version %q", baseRelease.Version))
			continue
		}
		baseSeen[baseRelease.Version] = true
		headRelease, ok := findReleaseByVersion(head, baseRelease.Version)
		if !ok {
			msgs = append(msgs, fmt.Sprintf("head is missing release %q", baseRelease.Version))
			continue
		}
		sort.Sort(releaseFileSorter(headRelease.Files))
		sort.Sort(releaseFileSorter(baseRelease.Files))

		if !cmp.Equal(baseRelease, headRelease) {
			msgs = append(msgs, fmt.Sprintf("release %q differs:\n%s",
				baseRelease.Version, cmp.Diff(baseRelease, headRelease)),
			)
		}
	}
	for _, headRelease := range head {
		if headRelease.Version == "" {
			msgs = append(msgs, "head has a release with no version")
			continue
		}
		if headSeen[headRelease.Version] {
			msgs = append(msgs, fmt.Sprintf("head has multiple releases with version %q", headRelease.Version))
			continue
		}
		headSeen[headRelease.Version] = true
	}
	return msgs
}
