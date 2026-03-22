# Monitoring Query Tool

Queries system and infrastructure monitoring data to analyze performance, resource usage, and health.

## Supported Systems
- **system-stats**: CPU, memory, load average, and top processes
- **disk-usage**: Filesystem usage, inode usage, and largest directories
- **process-top**: Process analysis (top by CPU/memory, zombies, open FDs)
- **docker-stats**: Container resource usage, disk usage, and image inventory
- **node-exporter**: Prometheus node exporter metrics (localhost:9100)
- **prometheus**: Direct PromQL queries against Prometheus API (localhost:9090)

## Usage
- Set `system` to choose the monitoring data source
- Set `query` for PromQL expressions when using `prometheus`
- Set `time_range` to control the lookback window (5m, 15m, 1h, 6h, 24h)
- Set `top_n` to limit the number of top results

## Security
- All queries are read-only
- Network access is only enabled for prometheus API queries
- Sandboxed execution with resource limits
- Full audit trail
