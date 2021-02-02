package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/willabides/goversions/goversion"
)

func main() {
	listenAddr := "127.0.0.1:9834"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		listenAddr = ":" + val
	}
	sMux := http.NewServeMux()
	sMux.Handle("/api/goversion-select", &goVersionSelectHandler{
		versionsMaxAge: 15 * time.Minute,
		versionsSource: "https://raw.githubusercontent.com/WillAbides/goreleases/main/versions.txt",
	})
	log.Printf("About to listen on %s. Try http://%s/api/goversion-select?constraint=1.x", listenAddr, listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, sMux))
}

type goVersionSelectHandler struct {
	versionsMaxAge time.Duration
	versionsSource string
	versionsMux    sync.Mutex
	versionsTime   time.Time
	versions       []*goversion.Version
}

func (h *goVersionSelectHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := req.URL.Query().Get("constraint")
	if c == "" {
		c = "1.x"
	}
	constraint, err := goversion.NewConstraints(c)
	if err != nil {
		http.Error(w, "invalid constraint", http.StatusBadRequest)
		return
	}
	versions, err := h.getVersions()
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var result *goversion.Version
	for _, v := range versions {
		if !constraint.Check(v) {
			continue
		}
		if result == nil || v.GreaterThan(result) {
			result = v
		}
	}
	if result == nil {
		http.Error(w, "no matching version found", http.StatusNotFound)
		return
	}
	fmt.Fprintln(w, result.String())
}

func (h *goVersionSelectHandler) getVersions() ([]*goversion.Version, error) {
	h.versionsMux.Lock()
	defer h.versionsMux.Unlock()
	needsRefresh := false
	if h.versionsTime.IsZero() {
		needsRefresh = true
	}
	if time.Since(h.versionsTime) > h.versionsMaxAge {
		needsRefresh = true
	}
	if !needsRefresh {
		return h.versions, nil
	}
	resp, err := http.Get(h.versionsSource)
	if err != nil {
		return h.versions, err
	}
	if resp.StatusCode != 200 {
		return h.versions, fmt.Errorf("not OK")
	}
	versions := make([]*goversion.Version, 0, len(h.versions))
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		l := strings.TrimSpace(scanner.Text())
		if l == "" {
			continue
		}
		var v *goversion.Version
		v, err = goversion.NewVersion(l)
		if err != nil {
			continue
		}
		versions = append(versions, v)
	}
	err = scanner.Err()
	if err != nil {
		return h.versions, err
	}
	h.versions = versions
	h.versionsTime = time.Now()
	return h.versions, resp.Body.Close()
}
