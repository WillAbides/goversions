package goreleases

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/dnaeon/go-vcr/recorder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	updateVCR    = flag.Bool("update-vcr", false, "Set to update VCR recordings")
	updateGolden = flag.Bool("write-golden", false, "Set to update golden files")
)

//go:generate go test . -write-golden

func vcr(t *testing.T, cassette string) *recorder.Recorder {
	t.Helper()
	mode := recorder.ModeReplaying
	if updateVCR != nil && *updateVCR {
		mode = recorder.ModeRecording
	}
	r, err := recorder.NewAsMode(cassette, mode, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, r.Stop())
	})
	return r
}

func testHTTPClient(t *testing.T, cassette string) *http.Client {
	t.Helper()
	if cassette == "" {
		cassette = "testdata/vcr/gldo_client_default"
	}
	return &http.Client{
		Transport: vcr(t, cassette),
	}
}

func TestFetchRelease(t *testing.T) {
	t.Run("golden", func(t *testing.T) {
		releases, err := FetchReleases(context.Background(), &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		encoded, err := json.MarshalIndent(&releases, "", " ")
		require.NoError(t, err)
		goldenFile := filepath.FromSlash("testdata/golden/releases.json")
		if updateGolden != nil && *updateGolden {
			require.NoError(t, os.MkdirAll(filepath.Dir(goldenFile), 0o700))
			err = os.WriteFile(goldenFile, encoded, 0o600)
			require.NoError(t, err)
		}
		want, err := os.ReadFile(goldenFile)
		require.NoError(t, err)
		require.Equal(t, string(want), string(encoded))
	})
}

func TestFindConflicts(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		ctx := context.Background()
		baseReleases, err := FetchReleases(ctx, &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		headReleases, err := FetchReleases(ctx, &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		got := FindConflicts(baseReleases, headReleases)
		require.Empty(t, got)
	})

	t.Run("missing release", func(t *testing.T) {
		ctx := context.Background()
		baseReleases, err := FetchReleases(ctx, &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		headReleases, err := FetchReleases(ctx, &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		headReleases = headReleases[1:]
		got := FindConflicts(baseReleases, headReleases)
		want := []string{`head is missing release "go1.17"`}
		require.Equal(t, want, got)
	})
}
