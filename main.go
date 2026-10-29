package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

const name = "gitea_exporter"

type config struct {
	listenAddress string
	telemetryPath string
	giteaURL      string
	tokenFile     string
	pollInterval  time.Duration
}

func main() {
	var cfg config
	showVersion := flag.Bool("version", false, "Print version information and exit.")
	flag.StringVar(&cfg.listenAddress, "web.listen-address", ":8080", "Address on which to expose metrics and the landing page.")
	flag.StringVar(&cfg.telemetryPath, "web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	flag.StringVar(&cfg.giteaURL, "gitea.url", "", "Base URL of the Gitea instance (required).")
	flag.StringVar(&cfg.tokenFile, "gitea.token-file", "", "File holding a Gitea token with read:admin and read:repository scopes (required).")
	flag.DurationVar(&cfg.pollInterval, "gitea.poll-interval", 5*time.Minute, "How often to poll the Gitea API.")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Print(name))
		return
	}
	if cfg.giteaURL == "" || cfg.tokenFile == "" {
		fmt.Fprintln(os.Stderr, "--gitea.url and --gitea.token-file are required")
		flag.Usage()
		os.Exit(2)
	}

	if err := run(cfg); err != nil {
		slog.Error("gitea_exporter failed", "err", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	token, err := os.ReadFile(cfg.tokenFile)
	if err != nil {
		return err
	}
	c := &client{
		base:  strings.TrimRight(cfg.giteaURL, "/"),
		token: strings.TrimSpace(string(token)),
		http:  &http.Client{Timeout: 30 * time.Second},
	}
	if c.token == "" {
		return errors.New("Gitea token is empty")
	}

	col := newCollector()
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		col,
		versioncollector.NewCollector(name),
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	go every(ctx, cfg.pollInterval, sourceIssues, col.errors, func(ctx context.Context) error {
		s, err := pollIssues(ctx, c)
		if err != nil {
			return err
		}
		col.issues.Store(s)
		return nil
	})
	go every(ctx, cfg.pollInterval, sourceWorkflows, col.errors, func(ctx context.Context) error {
		s, err := pollWorkflows(ctx, c, col.workflows.Load())
		if err != nil {
			return err
		}
		col.workflows.Store(s)
		return nil
	})

	landing, err := landingPage(cfg.telemetryPath)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("GET "+cfg.telemetryPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		ErrorLog:      slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
		ErrorHandling: promhttp.ContinueOnError,
	}))
	mux.Handle("GET /{$}", landing)
	srv := &http.Server{
		Addr:              cfg.listenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("serving metrics", "addr", srv.Addr, "version", version.Info(), "poll_interval", cfg.pollInterval)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

var landingTemplate = template.Must(template.New("landing").Parse(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>{{.Name}}</title></head>
<body>
<h1>{{.Name}}</h1>
<p>Prometheus exporter for Gitea pull requests and Actions workflow runs.</p>
<p>Version: {{.Version}}</p>
<ul><li><a href="{{.MetricsPath}}">Metrics</a></li></ul>
</body>
</html>
`))

func landingPage(metricsPath string) (http.Handler, error) {
	var b strings.Builder
	err := landingTemplate.Execute(&b, struct{ Name, Version, MetricsPath string }{name, version.Version, metricsPath})
	if err != nil {
		return nil, err
	}
	page := b.String()
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, page)
	}), nil
}

func every(ctx context.Context, interval time.Duration, source string, failures *prometheus.CounterVec, poll func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		pollCtx, cancel := context.WithTimeout(ctx, interval)
		start := time.Now()
		err := poll(pollCtx)
		cancel()
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			failures.WithLabelValues(source).Inc()
			slog.Error("update failed", "source", source, "err", err)
		default:
			slog.Info("updated", "source", source, "duration", time.Since(start).Round(time.Millisecond))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
