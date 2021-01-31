package versions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultAPIBase = `https://storage.googleapis.com`
	defaultBucket  = `golang`
	defaultPrefix  = `go1`
)

type storageClient struct {
	baseURL    string
	bucket     string
	prefix     string
	httpClient *http.Client
}

func (c *storageClient) getBaseURL() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return defaultAPIBase
}

func (c *storageClient) getBucket() string {
	if c.bucket != "" {
		return c.bucket
	}
	return defaultBucket
}

func (c *storageClient) getPrefix() string {
	if c.prefix != "" {
		return c.prefix
	}
	return defaultPrefix
}

func (c *storageClient) getHTTPClient() *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return http.DefaultClient
}

func (c *storageClient) doContentRequest(ctx context.Context, objectName string) (*http.Response, error) {
	u := fmt.Sprintf(`%s/%s/%s`, c.getBaseURL(), c.getBucket(), objectName)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	return c.getHTTPClient().Do(req)
}

func (c *storageClient) fetchStorageObjects(ctx context.Context) ([]storageObject, error) {
	tkn := ""
	baseURL := fmt.Sprintf(`%s/storage/v1/b/%s/o?prefix=%s`, c.getBaseURL(), c.getBucket(), c.getPrefix())
	var objects []storageObject
	for {
		req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"&pageToken="+tkn, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.getHTTPClient().Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("not OK")
		}
		pg, err := readPage(resp.Body)
		if err != nil {
			return nil, err
		}
		err = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		objects = append(objects, pg.Items...)
		tkn = pg.NextPageToken
		if tkn == "" {
			break
		}
	}
	return objects, nil
}

func readPage(r io.Reader) (*resPage, error) {
	dec := json.NewDecoder(r)
	var pg resPage
	err := dec.Decode(&pg)
	if err != nil {
		return nil, err
	}
	return &pg, nil
}

type resPage struct {
	NextPageToken string          `json:"nextPageToken"`
	Items         []storageObject `json:"items"`
}

type storageObject struct {
	Name        string    `json:"name"`
	ETag        string    `json:"etag"`
	Size        string    `json:"size"`
	TimeCreated time.Time `json:"timeCreated"`
}

func (f storageObject) size() int64 {
	i, err := strconv.ParseInt(f.Size, 10, 64)
	if err != nil {
		return 0
	}
	return i
}
