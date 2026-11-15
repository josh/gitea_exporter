package main

import (
	"context"
	"time"
)

type runnersSnapshot struct {
	at      time.Time
	runners []actionRunner
}

func pollRunners(ctx context.Context, c *client) (*runnersSnapshot, error) {
	s := &runnersSnapshot{at: time.Now()}
	for runners, err := range pages(ctx, c, "/admin/actions/runners", nil, func(p actionRunners) []actionRunner { return p.Runners }) {
		if err != nil {
			return nil, err
		}
		for _, r := range runners {
			if !r.Disabled {
				s.runners = append(s.runners, r)
			}
		}
	}
	return s, nil
}
