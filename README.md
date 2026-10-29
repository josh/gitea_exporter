# gitea_exporter

Prometheus exporter for Gitea pull requests and Actions workflow runs.

It polls the Gitea API in the background and serves the latest snapshot on `/metrics`, so scrapes never wait on Gitea.

## Usage

```sh
gitea_exporter --gitea.url=http://gitea:3000 --gitea.token-file=/secrets/token
```

The token must belong to a site admin and needs the `read:admin` and `read:repository` scopes. `read:admin` covers `/admin/actions/runs`, and admin access lets repository search see private repositories.

| Flag | Default | Description |
| --- | --- | --- |
| `--web.listen-address` | `:8080` | Address on which to expose metrics and the landing page |
| `--web.telemetry-path` | `/metrics` | Path under which to expose metrics |
| `--gitea.url` | | Base URL of the Gitea instance (required) |
| `--gitea.token-file` | | File holding the Gitea token (required) |
| `--gitea.poll-interval` | `5m` | How often to poll the Gitea API |
| `--version` | | Print version information and exit |

## Metrics

| Metric | Labels | Description |
| --- | --- | --- |
| `gitea_open_issues` | `gitea_repo` | Open issues per repository |
| `gitea_open_pull_requests` | `gitea_repo`, `author` | Open pull requests per repository and author |
| `gitea_pull_request_commit_status` | `gitea_repo`, `pull_request`, `author`, `gitea_commit_status` | Combined commit status of each open pull request's head commit, one series per state set to 0 or 1 |
| `gitea_workflow_run_conclusion` | `gitea_repo`, `workflow_name`, `gitea_workflow_run_conclusion` | Conclusion of the newest completed run of each workflow on the default branch, one series per conclusion set to 0 or 1 |
| `gitea_workflow_run_number` | `gitea_repo`, `workflow_name` | Run number of that newest run |
| `gitea_exporter_last_success_timestamp_seconds` | `source` | Unix time of the last successful update of `issues` or `workflows` |
| `gitea_exporter_errors_total` | `source` | Failed updates |
| `gitea_exporter_build_info` | `version`, `revision`, `branch`, `goversion`, `goos`, `goarch`, `tags` | Build metadata |
