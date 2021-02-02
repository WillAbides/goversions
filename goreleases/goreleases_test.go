package goreleases

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

func testHTTPClient(t *testing.T, gcCassette, scCassette string) *http.Client {
	t.Helper()
	if gcCassette == "" {
		gcCassette = "testdata/vcr/gldo_client_default"
	}
	if scCassette == "" {
		scCassette = "testdata/vcr/storage_client_default"
	}
	return &http.Client{
		Transport: &testTransport{
			gcr: vcr(t, gcCassette),
			scr: vcr(t, scCassette),
		},
	}
}

type testTransport struct {
	gcr *recorder.Recorder
	scr *recorder.Recorder
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Host, "googleapis") {
		return t.scr.RoundTrip(req)
	}
	return t.gcr.RoundTrip(req)
}

func TestFetchRelease(t *testing.T) {
	t.Run("golden", func(t *testing.T) {
		releases, err := FetchReleases(context.Background(), &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, "", ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		encoded, err := json.MarshalIndent(&releases, "", " ")
		require.NoError(t, err)
		goldenFile := filepath.FromSlash("testdata/golden/releases.json")
		if updateGolden != nil && *updateGolden {
			require.NoError(t, os.MkdirAll(filepath.Dir(goldenFile), 0o700))
			err = ioutil.WriteFile(goldenFile, encoded, 0o600)
			require.NoError(t, err)
		}
		want, err := ioutil.ReadFile(goldenFile)
		require.NoError(t, err)
		require.Equal(t, string(want), string(encoded))
	})

	// sanity check that we have the same data as golang.org/dl/
	t.Run("gldo consistency", func(t *testing.T) {
		ctx := context.Background()
		releases, err := FetchReleases(ctx, &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, "", ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		gClient := &gldoClient{
			httpClient: testHTTPClient(t, "", ""),
		}
		gReleases, err := gClient.fetchReleases(ctx)
		require.NoError(t, err)

		for _, gRelease := range gReleases {
			// golang.org/dl has an errant go1 release. We can ignore it.
			if gRelease.Version == "go1" {
				continue
			}
			release, ok := findReleaseByVersion(releases, gRelease.Version)
			assert.True(t, ok, gRelease.Version)
			assert.Equal(t, gRelease.Version, release.Version, gRelease.Version)
			assert.Equal(t, gRelease.Stable, release.Stable, gRelease.Version)
			assert.Equal(t, len(gRelease.Files), len(release.Files), gRelease.Version)
			for _, gFile := range gRelease.Files {
				file, ok := findReleaseFileByName(release.Files, gFile.Filename)
				// because golang.org is missing sizes on some files
				if gFile.Size == 0 {
					gFile.Size = file.Size
				}
				assert.True(t, ok, gFile.Filename)
				assert.Equal(t, gFile, file, gFile.Filename)
			}
		}

		// make sure all our extra releases are unstable
		for _, release := range releases {
			if !release.Stable {
				continue
			}
			_, ok := findReleaseByVersion(gReleases, release.Version)
			assert.True(t, ok, release.Version)
		}
	})
}

func TestFindConflicts(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		ctx := context.Background()
		baseReleases, err := FetchReleases(ctx, &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, "", ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		headReleases, err := FetchReleases(ctx, &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, "", ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		got := FindConflicts(baseReleases, headReleases)
		require.Empty(t, got)
	})

	t.Run("missing release", func(t *testing.T) {
		ctx := context.Background()
		baseReleases, err := FetchReleases(ctx, &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, "", ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		headReleases, err := FetchReleases(ctx, &FetchReleasesOptions{
			HTTPClient:   testHTTPClient(t, "", ""),
			SkipVersions: []string{"go1.7.2"},
		})
		require.NoError(t, err)
		headReleases = headReleases[1:]
		got := FindConflicts(baseReleases, headReleases)
		want := []string{`head is missing release "go1.16rc1"`}
		require.Equal(t, want, got)
	})
}
