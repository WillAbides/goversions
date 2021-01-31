package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/willabides/goversions/goreleases"
)

type options struct {
	FetchReleases  fetchReleasesCmd  `kong:"cmd,name=fetch,default='1',help='fetch releases from the internet'"`
	CheckConflicts checkConflictsCmd `kong:"cmd,help='check that head does not have any conflicts with base that would prevent automatic merging'"`
}

type fetchReleasesCmd struct {
	Exclude []string `kong:"default='go1.7.2',help='Go versions to exclude. go1.7.2 is a default because it was retracted.'"`
}

func (x *fetchReleasesCmd) Run(k *kong.Context) error {
	ctx := context.Background()
	releases, err := goreleases.FetchReleases(ctx, &goreleases.FetchReleasesOptions{
		SkipVersions: x.Exclude,
	})
	if err != nil {
		return fmt.Errorf("couldn't build releases %v", err)
	}
	enc := json.NewEncoder(k.Stdout)
	enc.SetIndent("", " ")
	err = enc.Encode(&releases)
	if err != nil {
		return fmt.Errorf("couldn't encode releases %v", err)
	}
	return nil
}

type checkConflictsCmd struct {
	Base string `kong:"arg,help='path to the base file'"`
	Head string `kong:"arg,help='path to the head file'"`
}

func (x *checkConflictsCmd) Run(k *kong.Context) error {
	ok, diff, err := checkConflicts(x.Base, x.Head)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	fmt.Fprintf(k.Stdout, `found a conflict that prevents automatic merging:
%s
`, diff)
	k.Exit(1)
	return nil
}

func main() {
	var cli options
	k := kong.Parse(&cli)
	k.FatalIfErrorf(k.Run())
}
