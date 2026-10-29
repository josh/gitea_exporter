# AGENTS.md

Prometheus exporter for Gitea pull requests and Actions workflow runs. Single `main` package; dependencies are limited to client_golang and prometheus/common.

Before committing:

- `go fmt ./...`
- `go vet ./...`

Keep comments concise. Only add them when they clarify non-obvious logic.
