package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type client struct {
	endpoint, token string
	demo            bool
}

func (c client) get(ctx context.Context, path string, params url.Values, dst any) ([]string, error) {
	if c.demo {
		return demoGet(ctx, path, params, dst)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(c.endpoint, "/")+"/api/v1/"+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var env struct {
		Status   string
		Data     json.RawMessage
		Error    string
		Warnings []string
	}
	if err = json.Unmarshal(b, &env); err != nil {
		return nil, fmt.Errorf("HTTP %d: invalid JSON", resp.StatusCode)
	}
	if resp.StatusCode/100 != 2 || env.Status != "success" {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, env.Error)
	}
	return env.Warnings, json.Unmarshal(env.Data, dst)
}
func (c client) metadata(ctx context.Context, path string) ([]string, error) {
	var values []string
	_, e := c.get(ctx, path, nil, &values)
	return values, e
}
