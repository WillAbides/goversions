package main

import (
	"context"
	"encoding/json"

	"github.com/alecthomas/kong"
	"github.com/willabides/goversions/goreleases"
)

var cli struct {
	Exclude []string `kong:"default='go1.7.2',help='Go versions to exclude. go1.7.2 is a default because it was retracted.'"`
}

func main() {
	ctx := context.Background()
	k := kong.Parse(&cli)
	releases, err := goreleases.FetchReleases(ctx, &goreleases.FetchReleasesOptions{
		SkipVersions: cli.Exclude,
	})
	k.FatalIfErrorf(err, "couldn't build releases")
	enc := json.NewEncoder(k.Stdout)
	enc.SetIndent("", " ")
	err = enc.Encode(&releases)
	k.FatalIfErrorf(err, "couldn't encode releases")
}
