package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/willabides/goversions/goversion"
)

const description = `
goversion-select selects matching go versions from a list.

For example, get the newest version of go 1.15 like so:

  curl -s 'https://raw.githubusercontent.com/WillAbides/goreleases/main/versions.txt' \
    | goversion-select -i -c '1.15' -
`

var version = "unknown"

var cli struct {
	Version            kong.VersionFlag `kong:"short=v,help='output goversion-select version and exit'"`
	Constraint         string           `kong:"required,short=c,help='constraint to match'"`
	MaxResults         int              `kong:"short=n,help='maximum number of results to output'"`
	IgnoreInvalid      bool             `kong:"short=i,help='ignore invalid candidates instead of erroring'"`
	ValidateConstraint bool             `kong:"help='just validate the constraint. exits non-zero if invalid'"`
	Candidates         []string         `kong:"arg,help='candidate versions to consider -- value of \"-\" indicates stdin'"`
}

func getVersions(args []string, stdin io.Reader, ignore bool) ([]*goversion.Version, error) {
	res := make([]*goversion.Version, 0, len(args))
	doStdin := false
	var err error
	for _, arg := range args {
		if arg == "-" {
			doStdin = true
			break
		}
		res, err = addVersion(arg, ignore, res)
		if err != nil {
			return nil, err
		}
	}
	if !doStdin {
		return res, nil
	}
	r := bufio.NewScanner(stdin)
	for r.Scan() {
		res, err = addVersion(r.Text(), ignore, res)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func addVersion(ver string, ignore bool, versions []*goversion.Version) ([]*goversion.Version, error) {
	v, err := goversion.NewVersion(ver)
	if err != nil {
		if ignore {
			return versions, nil
		}
		return nil, fmt.Errorf("could not parse version %q: %v", ver, err)
	}
	return append(versions, v), nil
}

func main() {
	k := kong.Parse(&cli,
		kong.Vars{"version": version},
		kong.Description(strings.TrimSpace(description)),
	)

	c, err := goversion.NewConstraints(cli.Constraint)
	if cli.ValidateConstraint {
		if err != nil {
			fmt.Fprintf(k.Stderr, "invalid constraint: %q\n", cli.Constraint)
			k.Exit(1)
		}
		fmt.Println(c)
		k.Exit(0)
	}
	k.FatalIfErrorf(err)

	versions, err := getVersions(cli.Candidates, os.Stdin, cli.IgnoreInvalid)
	k.FatalIfErrorf(err)

	for _, s := range results(c, cli.MaxResults, versions) {
		fmt.Println(s)
	}
}

func results(c *goversion.Constraints, max int, versions []*goversion.Version) []string {
	candidates := make([]*goversion.Version, 0, len(versions))
	for _, v := range versions {
		if c.Check(v) {
			candidates = append(candidates, v)
		}
	}
	sort.Sort(sort.Reverse(goversion.Collection(candidates)))
	if max > 0 && max < len(candidates) {
		candidates = candidates[:max]
	}
	result := make([]string, len(candidates))
	for i, candidate := range candidates {
		result[i] = candidate.String()
	}
	return result
}
