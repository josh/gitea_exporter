package main

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

type pullKey struct{ repo, author string }

type prStatus struct{ repo, number, author, state string }

type issuesSnapshot struct {
	at       time.Time
	issues   map[string]int
	pulls    map[pullKey]int
	statuses []prStatus
}

func pollIssues(ctx context.Context, c *client) (*issuesSnapshot, error) {
	s := &issuesSnapshot{at: time.Now(), issues: map[string]int{}, pulls: map[pullKey]int{}}
	for repos, err := range pages(ctx, c, "/repos/search", url.Values{"archived": {"false"}}, func(p repoSearch) []repository { return p.Data }) {
		if err != nil {
			return nil, err
		}
		for _, repo := range repos {
			s.issues[repo.FullName] = repo.OpenIssuesCount
			if repo.OpenPRCounter == 0 {
				continue
			}
			if err := s.addPulls(ctx, c, repo.FullName); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

func (s *issuesSnapshot) addPulls(ctx context.Context, c *client, repo string) error {
	for pulls, err := range pages(ctx, c, "/repos/"+repo+"/pulls", url.Values{"state": {"open"}}, func(p []pullRequest) []pullRequest { return p }) {
		if err != nil {
			return err
		}
		for _, pr := range pulls {
			s.pulls[pullKey{repo, pr.User.Login}]++
			var status combinedStatus
			if err := c.get(ctx, "/repos/"+repo+"/commits/"+pr.Head.SHA+"/status", nil, &status); err != nil {
				return err
			}
			if status.TotalCount > 0 {
				s.statuses = append(s.statuses, prStatus{repo, strconv.Itoa(pr.Number), pr.User.Login, status.State})
			}
		}
	}
	return nil
}
