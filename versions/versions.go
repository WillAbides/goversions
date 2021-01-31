package versions

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/killa-beez/gopkgs/pool"
)

// FetchReleasesOptions options for FetchReleases
type FetchReleasesOptions struct {
	HTTPClient   *http.Client
	SkipVersions []string // go versions to skip ( like go1.7.2 which was pulled )
}

// FetchReleases fetches release data from storage.googleapis.com/golang and golang.org/dl
func FetchReleases(ctx context.Context, options *FetchReleasesOptions) ([]Release, error) {
	httpClient := http.DefaultClient
	if options == nil {
		options = new(FetchReleasesOptions)
	}
	if options.HTTPClient != nil {
		httpClient = options.HTTPClient
	}
	sc := &storageClient{
		httpClient: httpClient,
	}
	gc := &gldoClient{
		httpClient: httpClient,
	}
	objects, err := sc.fetchStorageObjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("error retrieving storage objects: %v", err)
	}
	releaseFiles, err := buildReleaseFiles(objects, options.SkipVersions)
	if err != nil {
		return nil, fmt.Errorf("error building release files: %v", err)
	}
	err = setShas(ctx, releaseFiles, gc, sc)
	if err != nil {
		return nil, fmt.Errorf("error getting shas: %v", err)
	}
	releases, err := buildReleases(releaseFiles)
	if err != nil {
		return nil, fmt.Errorf("error building releases: %v", err)
	}
	return releases, nil
}

// Release is a go release
type Release struct {
	Version string        `json:"version"`
	Stable  bool          `json:"stable"`
	Files   []ReleaseFile `json:"files"`
}

func (f Release) less(other Release) bool {
	var fVersion, otherVersion goVersion
	parseGoVersion(&fVersion, f.Version)
	parseGoVersion(&otherVersion, other.Version)
	return fVersion.less(otherVersion)
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

//nolint:gocritic // I don't want to refactor this to a pointer
func (f ReleaseFile) less(other ReleaseFile) bool {
	var fVersion, otherVersion goVersion
	parseGoVersion(&fVersion, f.Version)
	parseGoVersion(&otherVersion, other.Version)
	if fVersion.less(otherVersion) {
		return true
	}
	if otherVersion.less(fVersion) {
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

var ignorables = []*regexp.Regexp{
	regexp.MustCompile(`\.asc\z`),
	regexp.MustCompile(`\.sha256\z`),
	regexp.MustCompile(`-bootstrap-`),
}

func ignorableObject(object storageObject) bool {
	for _, ignorable := range ignorables {
		if ignorable.MatchString(object.Name) {
			return true
		}
	}
	return false
}

func buildReleases(releaseFiles []ReleaseFile) ([]Release, error) {
	rMap := map[string]*Release{}
	for _, rf := range releaseFiles {
		if rMap[rf.Version] == nil {
			rMap[rf.Version] = &Release{
				Version: rf.Version,
				Stable:  isStable(rf.Version),
			}
		}
		rMap[rf.Version].Files = append(rMap[rf.Version].Files, rf)
	}
	releases := make([]Release, 0, len(rMap))
	for _, release := range rMap {
		if release == nil {
			continue
		}
		releases = append(releases, *release)
	}
	sort.Sort(sort.Reverse(releaseSorter(releases)))
	return releases, nil
}

func skipVersion(version string, skips []string) bool {
	for _, skip := range skips {
		if skip == version {
			return true
		}
	}
	return false
}

func buildReleaseFiles(objects []storageObject, skipVersions []string) ([]ReleaseFile, error) {
	result := make([]ReleaseFile, 0, len(objects))
	for _, object := range objects {
		if ignorableObject(object) {
			continue
		}
		got, err := parseFilename(object.Name)
		if err != nil {
			return nil, err
		}
		version := "go" + got.version
		if skipVersion(version, skipVersions) {
			continue
		}
		result = append(result, ReleaseFile{
			Filename: got.name,
			OS:       got.os,
			Arch:     got.arch,
			Version:  "go" + got.version,
			Size:     object.size(),
			Kind:     got.kind,
		})
	}
	sort.Sort(sort.Reverse(releaseFileSorter(result)))
	return result, nil
}

type filenameInfo struct {
	name    string
	version string
	kind    string
	os      string
	arch    string
	suffix  string
}

type goVersion struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func (v goVersion) less(other goVersion) bool {
	if v.major < other.major {
		return true
	}
	if v.major > other.major {
		return false
	}
	if v.minor < other.minor {
		return true
	}
	if v.minor > other.minor {
		return false
	}
	if v.patch < other.patch {
		return true
	}
	if v.patch > other.patch {
		return false
	}
	if v.prerelease != "" && other.prerelease == "" {
		return true
	}
	if v.prerelease == "" && other.prerelease != "" {
		return false
	}
	return v.prerelease < other.prerelease
}

const versionPattern = `(\d+)(?:\.(\d+))?(?:\.(\d+))?([[:alnum:]]+)?`

var versionRegexp = regexp.MustCompile(versionPattern)

func parseGoVersion(dst *goVersion, version string) bool {
	parts := versionRegexp.FindStringSubmatch(version)
	if len(parts) == 0 {
		return false
	}
	if n, err := strconv.Atoi(parts[1]); err == nil {
		dst.major = n
	}
	if n, err := strconv.Atoi(parts[2]); err == nil {
		dst.minor = n
	}
	if n, err := strconv.Atoi(parts[3]); err == nil {
		dst.patch = n
	}
	dst.prerelease = parts[4]
	return true
}

var (
	installerFileExp = regexp.MustCompile(`\Ago(\d+(?:\.\d+)?(?:\.\d+)?(?:\w[[:alnum:]]*)?)\.([[:alnum:]]+)-([[:alnum:]]+)(?:-osx10\.\d)?((?:\..+)?(?:\.msi|\.pkg))\z`)
	archiveFileExp   = regexp.MustCompile(`\Ago(\d+(?:\.\d+)?(?:\.\d+)?(?:\w[[:alnum:]]*)?)\.([[:alnum:]]+)-([[:alnum:]]+)(?:-osx10\.\d)?(\..+)\z`)
	srcFileExp       = regexp.MustCompile(`\Ago(\d+(?:\.\d+)?(?:\.\d+)?(?:\w[[:alnum:]]*)?)(\.src.tar.gz.*)\z`)
)

func parseFilename(name string) (*filenameInfo, error) {
	if installerFileExp.MatchString(name) {
		m := installerFileExp.FindAllStringSubmatch(name, -1)
		return &filenameInfo{
			name:    name,
			kind:    "installer",
			version: m[0][1],
			os:      m[0][2],
			arch:    m[0][3],
			suffix:  m[0][4],
		}, nil
	}
	if archiveFileExp.MatchString(name) {
		m := archiveFileExp.FindAllStringSubmatch(name, -1)
		return &filenameInfo{
			name:    name,
			kind:    "archive",
			version: m[0][1],
			os:      m[0][2],
			arch:    m[0][3],
			suffix:  m[0][4],
		}, nil
	}
	if srcFileExp.MatchString(name) {
		m := srcFileExp.FindAllStringSubmatch(name, -1)
		return &filenameInfo{
			name:    name,
			kind:    "source",
			version: m[0][1],
			suffix:  m[0][2],
		}, nil
	}
	return nil, fmt.Errorf("no match for %q", name)
}

func setShas(ctx context.Context, files []ReleaseFile, gc *gldoClient, sc *storageClient) error {
	err := shasFromGldo(ctx, files, gc)
	if err != nil {
		return err
	}
	return shasFromStorage(ctx, files, sc)
}

func shasFromGldo(ctx context.Context, files []ReleaseFile, gc *gldoClient) error {
	releases, err := gc.fetchReleases(ctx)
	if err != nil {
		return err
	}
	gldoShas := map[string]string{}
	for _, release := range releases {
		for _, rf := range release.Files {
			if rf.Sha256 == "" {
				continue
			}
			gldoShas[rf.Filename] = rf.Sha256
		}
	}
	for i := range files {
		if files[i].Sha256 != "" {
			continue
		}
		files[i].Sha256 = gldoShas[files[i].Filename]
	}
	return nil
}

func shasFromStorage(ctx context.Context, files []ReleaseFile, sc *storageClient) error {
	p := pool.New(len(files), 160)
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	var mux sync.Mutex
	var err error
	for i := range files {
		i := i
		if files[i].Sha256 != "" {
			continue
		}
		p.Add(pool.NewWorkUnit(func(c context.Context) {
			sha, shaErr := getSha(c, files[i].Filename, sc)
			if shaErr != nil {
				mux.Lock()
				err = shaErr
				cancel()
				mux.Unlock()
			}
			files[i].Sha256 = sha
		}))
	}
	p.Start(ctx)
	p.Wait()
	return err
}

func getSha(ctx context.Context, name string, sc *storageClient) (string, error) {
	resp, err := sc.doContentRequest(ctx, name+".sha256")
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case 200:
	case 404:
		return "", nil
	default:
		return "", fmt.Errorf("not OK: %s", resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	err = resp.Body.Close()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func isStable(v string) bool {
	return !strings.Contains(v, "beta") && !strings.Contains(v, "rc")
}