# Metrics Collection & Prometheus Exposition

> Collection-level metrics support added in v0.1.4 (ISSUE-076).

`dbexplain` collects structured metrics from the schema collection pipeline — per-DSN timing, success/failure status, and table counts — and exposes them in two formats:

1. **JSON** — embedded in the standard JSON output (`"metrics"` field)
2. **Prometheus text format** — via the `--metrics` flag (output to stderr)

## Collected Metrics

Each DSN collection produces a metric snapshot with:

| Field | Type | Description |
|-------|------|-------------|
| `label` | string | DSN label from config |
| `kind` | string | Data source kind (mysql, redis, etc.) |
| `success` | bool | Whether collection succeeded |
| `duration_ms` | int64 | Collection wall-clock time in milliseconds |
| `num_databases` | int | Number of databases/schemas found |
| `num_tables` | int | Total tables collected |
| `error` | string | Error message (omitted on success) |

## JSON Output

Metrics are included in the standard JSON output under a top-level `"metrics"` field:

```bash
dbexplain --json
```

```json
{
  "instances": [...],
  "refs": [...],
  "groups": [...],
  "issues": [...],
  "metrics": [
    {
      "label": "my-mysql",
      "kind": "mysql",
      "success": true,
      "duration_ms": 1234,
      "num_databases": 1,
      "num_tables": 42
    },
    {
      "label": "my-redis",
      "kind": "redis",
      "success": false,
      "duration_ms": 5000,
      "num_databases": 0,
      "num_tables": 0,
      "error": "connection refused"
    }
  ]
}
```

This is backward-compatible — existing consumers ignore unknown fields.

## Prometheus Text Format

Use `--metrics` to output Prometheus-compatible text format to **stderr**:

```bash
dbexplain --metrics
dbexplain --json --human --metrics   # stdout=JSON, stderr=Prometheus
dbexplain collect --metrics          # also works with collect subcommand
```

Stdout remains pure (JSON or human-readable text). Prometheus text always goes to stderr.

### Example output

```
# HELP dbexplain_collect_duration_ms Collection duration per DSN
# TYPE dbexplain_collect_duration_ms gauge
dbexplain_collect_duration_ms{label="my-mysql",kind="mysql"} 1234
# HELP dbexplain_collect_success Collection success (1=success, 0=failure)
# TYPE dbexplain_collect_success gauge
dbexplain_collect_success{label="my-mysql",kind="mysql"} 1
# HELP dbexplain_collect_tables_total Total tables collected per DSN
# TYPE dbexplain_collect_tables_total gauge
dbexplain_collect_tables_total{label="my-mysql",kind="mysql"} 42
# HELP dbexplain_collect_duration_seconds Collection wall-clock duration
# TYPE dbexplain_collect_duration_seconds gauge
dbexplain_collect_duration_seconds 5.670
# HELP dbexplain_collect_success_total Total collections by status
# TYPE dbexplain_collect_success_total gauge
dbexplain_collect_success_total{status="success"} 1
dbexplain_collect_success_total{status="failure"} 1
# HELP dbexplain_collect_tables_total_all Total tables across all DSNs
# TYPE dbexplain_collect_tables_total_all gauge
dbexplain_collect_tables_total_all 42
```

### Pipeline to Prometheus Pushgateway

```bash
dbexplain --metrics | curl -X POST --data-binary @- \
  http://prometheus-pushgateway:9091/metrics/job/dbexplain
```

Note: all metrics are **gauge** type (one-shot CLI semantics). `rate()` / `increase()` PromQL functions are not applicable.

## Use Cases

- **Collection health monitoring**: track per-DSN success/failure over time
- **Performance regression detection**: duration_ms trending up indicates network/DB issues
- **Schema size tracking**: tables_total growing over time helps capacity planning
- **CI/CD pipeline validation**: fail build if any DSN collection fails (parse `success_total{status="failure"}`)

## Implementation

Metrics are collected in `main.go`'s collection goroutines (both `main()` and `handleCollect()`), aggregated in `src/internal/metrics/collect.go`, embedded in `analyze.Result.Metrics`, and serialized in `render.jsonResult.Metrics`.

Metrics are **not** collected for query execution (`dbexplain execute`) — that is scoped for a future release (TBD).
