package main

import (
	"context"
	"maps"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	initialLookback = 90 * 24 * time.Hour
	rescanOverlap   = 24 * time.Hour
)

type workflowKey struct{ repo, workflow string }

type workflowsSnapshot struct {
	at     time.Time
	latest map[workflowKey]workflowRun
}

func pollWorkflows(ctx context.Context, c *client, prev *workflowsSnapshot) (*workflowsSnapshot, error) {
	s := &workflowsSnapshot{at: time.Now(), latest: map[workflowKey]workflowRun{}}
	cutoff := s.at.Add(-initialLookback)
	if prev != nil {
		cutoff = prev.at.Add(-rescanOverlap)
		maps.Copy(s.latest, prev.latest)
	}
	for runs, err := range pages(ctx, c, "/admin/actions/runs", url.Values{"status": {"completed"}}, func(p workflowRuns) []workflowRun { return p.WorkflowRuns }) {
		if err != nil {
			return nil, err
		}
		recent := false
		for _, run := range runs {
			if run.UpdatedAt.After(cutoff) {
				recent = true
			}
			if run.Repository.Archived || run.HeadBranch != run.Repository.DefaultBranch {
				continue
			}
			key := workflowKey{run.Repository.FullName, workflowName(run.Path)}
			if cur, ok := s.latest[key]; !ok || newer(run, cur) {
				s.latest[key] = run
			}
		}
		if !recent {
			break
		}
	}
	return s, nil
}

func workflowName(p string) string {
	file, _, _ := strings.Cut(p, "@")
	return path.Base(file)
}

func newer(a, b workflowRun) bool {
	if a.RunNumber != b.RunNumber {
		return a.RunNumber > b.RunNumber
	}
	return a.RunAttempt > b.RunAttempt
}
