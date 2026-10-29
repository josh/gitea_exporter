package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const pageSize = 50

type client struct {
	base  string
	token string
	http  *http.Client
}

func (c *client) get(ctx context.Context, path string, query url.Values, v any) error {
	u := c.base + "/api/v1" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET %s: %s: %s", path, resp.Status, bytes.TrimSpace(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	return nil
}

func pages[P, T any](ctx context.Context, c *client, path string, query url.Values, items func(P) []T) iter.Seq2[[]T, error] {
	return func(yield func([]T, error) bool) {
		q := url.Values{}
		for k, v := range query {
			q[k] = v
		}
		q.Set("limit", strconv.Itoa(pageSize))
		for page := 1; ; page++ {
			q.Set("page", strconv.Itoa(page))
			var p P
			if err := c.get(ctx, path, q, &p); err != nil {
				yield(nil, err)
				return
			}
			batch := items(p)
			if len(batch) == 0 || !yield(batch, nil) || len(batch) < pageSize {
				return
			}
		}
	}
}

type repository struct {
	FullName        string `json:"full_name"`
	Archived        bool   `json:"archived"`
	DefaultBranch   string `json:"default_branch"`
	OpenIssuesCount int    `json:"open_issues_count"`
	OpenPRCounter   int    `json:"open_pr_counter"`
}

type repoSearch struct {
	Data []repository `json:"data"`
}

type pullRequest struct {
	Number int `json:"number"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type combinedStatus struct {
	State      string `json:"state"`
	TotalCount int    `json:"total_count"`
}

type workflowRun struct {
	Path       string     `json:"path"`
	HeadBranch string     `json:"head_branch"`
	Conclusion string     `json:"conclusion"`
	RunNumber  int64      `json:"run_number"`
	RunAttempt int64      `json:"run_attempt"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Repository repository `json:"repository"`
}

type workflowRuns struct {
	WorkflowRuns []workflowRun `json:"workflow_runs"`
}
