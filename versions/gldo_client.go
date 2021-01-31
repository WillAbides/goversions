package versions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type gldoClient struct {
	httpClient *http.Client
}

func (c *gldoClient) fetchReleases(ctx context.Context) ([]Release, error) {
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	u := `https://golang.org/dl/?mode=json&include=all`
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("not OK")
	}
	var result []Release
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
