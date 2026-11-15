package main

import (
	"strconv"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	sourceIssues    = "issues"
	sourceWorkflows = "workflows"
	sourceRunners   = "runners"
)

var (
	commitStatuses = []string{"pending", "success", "error", "failure", "warning", "skipped"}
	conclusions    = []string{"success", "failure", "cancelled", "skipped"}

	openIssuesDesc = prometheus.NewDesc(
		"gitea_open_issues",
		"Open issues per repository.",
		[]string{"gitea_repo"}, nil,
	)
	openPullRequestsDesc = prometheus.NewDesc(
		"gitea_open_pull_requests",
		"Open pull requests per repository and author.",
		[]string{"gitea_repo", "author"}, nil,
	)
	prStatusDesc = prometheus.NewDesc(
		"gitea_pull_request_commit_status",
		"Combined commit status of the head commit of each open pull request.",
		[]string{"gitea_repo", "pull_request", "author", "gitea_commit_status"}, nil,
	)
	runConclusionDesc = prometheus.NewDesc(
		"gitea_workflow_run_conclusion",
		"Conclusion of the newest completed workflow run on the default branch.",
		[]string{"gitea_repo", "workflow_name", "gitea_workflow_run_conclusion"}, nil,
	)
	runNumberDesc = prometheus.NewDesc(
		"gitea_workflow_run_number",
		"Run number of the newest completed workflow run on the default branch.",
		[]string{"gitea_repo", "workflow_name"}, nil,
	)
	runnerOnlineDesc = prometheus.NewDesc(
		"gitea_actions_runner_online",
		"Whether each enabled Actions runner is online.",
		[]string{"runner_id", "runner_name"}, nil,
	)
	lastSuccessDesc = prometheus.NewDesc(
		"gitea_exporter_last_success_timestamp_seconds",
		"Unix time of the last successful update per source.",
		[]string{"source"}, nil,
	)
)

type collector struct {
	issues    atomic.Pointer[issuesSnapshot]
	workflows atomic.Pointer[workflowsSnapshot]
	runners   atomic.Pointer[runnersSnapshot]
	errors    *prometheus.CounterVec
}

func newCollector() *collector {
	c := &collector{
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gitea_exporter_errors_total",
			Help: "Failed updates per source.",
		}, []string{"source"}),
	}
	c.errors.WithLabelValues(sourceIssues)
	c.errors.WithLabelValues(sourceWorkflows)
	c.errors.WithLabelValues(sourceRunners)
	return c
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- openIssuesDesc
	ch <- openPullRequestsDesc
	ch <- prStatusDesc
	ch <- runConclusionDesc
	ch <- runNumberDesc
	ch <- runnerOnlineDesc
	ch <- lastSuccessDesc
	c.errors.Describe(ch)
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	if s := c.issues.Load(); s != nil {
		for repo, n := range s.issues {
			ch <- prometheus.MustNewConstMetric(openIssuesDesc, prometheus.GaugeValue, float64(n), repo)
		}
		for k, n := range s.pulls {
			ch <- prometheus.MustNewConstMetric(openPullRequestsDesc, prometheus.GaugeValue, float64(n), k.repo, k.author)
		}
		for _, pr := range s.statuses {
			for _, state := range commitStatuses {
				ch <- prometheus.MustNewConstMetric(prStatusDesc, prometheus.GaugeValue, oneHot(pr.state == state), pr.repo, pr.number, pr.author, state)
			}
		}
		ch <- prometheus.MustNewConstMetric(lastSuccessDesc, prometheus.GaugeValue, float64(s.at.Unix()), sourceIssues)
	}
	if s := c.workflows.Load(); s != nil {
		for k, run := range s.latest {
			for _, conclusion := range conclusions {
				ch <- prometheus.MustNewConstMetric(runConclusionDesc, prometheus.GaugeValue, oneHot(run.Conclusion == conclusion), k.repo, k.workflow, conclusion)
			}
			ch <- prometheus.MustNewConstMetric(runNumberDesc, prometheus.GaugeValue, float64(run.RunNumber), k.repo, k.workflow)
		}
		ch <- prometheus.MustNewConstMetric(lastSuccessDesc, prometheus.GaugeValue, float64(s.at.Unix()), sourceWorkflows)
	}
	if s := c.runners.Load(); s != nil {
		for _, r := range s.runners {
			ch <- prometheus.MustNewConstMetric(runnerOnlineDesc, prometheus.GaugeValue, oneHot(r.Status == "online"), strconv.FormatInt(r.ID, 10), r.Name)
		}
		ch <- prometheus.MustNewConstMetric(lastSuccessDesc, prometheus.GaugeValue, float64(s.at.Unix()), sourceRunners)
	}
	c.errors.Collect(ch)
}

func oneHot(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
