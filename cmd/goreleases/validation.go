package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/willabides/goversions/goreleases"
)

func checkConflicts(baseFilename, headFilename string) (ok bool, msg string, err error) {
	var base, head []goreleases.Release
	baseBytes, err := ioutil.ReadFile(baseFilename) //nolint:gosec // checked
	if err != nil {
		return false, "", fmt.Errorf("error reading file %q: %v", baseFilename, err)
	}
	err = json.Unmarshal(baseBytes, &base)
	if err != nil {
		return false, "", fmt.Errorf("error unmarshaling file %q: %v", baseFilename, err)
	}
	headBytes, err := ioutil.ReadFile(headFilename) //nolint:gosec // checked
	if err != nil {
		return false, "", fmt.Errorf("error reading file %q: %v", headFilename, err)
	}
	err = json.Unmarshal(headBytes, &head)
	if err != nil {
		return false, "", fmt.Errorf("error unmarshaling file %q: %v", headFilename, err)
	}

	conflicts := goreleases.FindConflicts(base, head)
	if len(conflicts) == 0 {
		return true, "", nil
	}
	return false, strings.Join(conflicts, "\n"), nil
}
