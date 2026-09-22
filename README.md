# Kubernetes InfraOps Platform

`kubernetes-infraops-platform` is a server/agent operations platform for Kubernetes and host infrastructure. The server owns asset data, the service tree, task dispatch, target distribution, result aggregation, and Prometheus metrics. Agents execute tasks, collect log metrics, and run protocol probes.

## Multi-protocol probing

The `protocol-prober` role is consolidated into `pkg/prober` instead of running as a second server/agent stack. The integrated engine supports `icmp`, `http`, `tcp`, `dns`, and `tls`. The server continues to distribute targets over the existing RPC connection; the agent runs probes through a bounded worker pool and returns the latest samples.

```yaml
probe:
  - type: http
    region: external
    interval_seconds: 15
    timeout_seconds: 5
    target:
      - https://example.com/health
    options:
      method: GET

  - type: tcp
    region: database
    target:
      - mysql.example.internal:3306

  - type: dns
    region: external
    target:
      - example.com
    options:
      server: 8.8.8.8:53

  - type: tls
    region: external
    target:
      - example.com:443
    options:
      server_name: example.com

  - type: icmp
    region: external
    target:
      - 1.1.1.1
    options:
      count: "3"
      privileged: "false"
```

Every sample is exposed centrally through `infraops_probe_value` with protocol, worker, address, and region labels. Existing ICMP and HTTP metric names remain available; TCP, DNS, and TLS also expose protocol-specific metrics. Agents expose their latest local samples through `infraops_agent_probe_value`.

## Prometheus HTTP service discovery

The server exposes resources mounted in the service tree as Prometheus HTTP SD target groups:

```yaml
scrape_configs:
  - job_name: infraops-service-tree
    http_sd_configs:
      - url: http://infraops-server:8086/apis/v1/prometheus/http-sd
        refresh_interval: 30s
```

`/http-sd` is retained as a compatibility alias for the original standalone `http-sd` project. Both endpoints return only `resource_host` records mounted at a complete `group.product.app` path. The default target is each private address on port `9100`.

The discovery URL accepts `service_tree`, `group`, `product`, `app`, `region`, `status`, `network=private|public|all`, `port`, and `job` query parameters. `service_tree` uses the complete `group.product.app` form and cannot be mixed with its three individual filters.

Host tags can override the scrape settings:

```json
{
  "prometheus.io/scrape": "true",
  "prometheus.io/port": "9200",
  "prometheus.io/path": "/metrics",
  "prometheus.io/scheme": "http",
  "prometheus.io/job": "node-exporter",
  "cluster": "production-a"
}
```

Non-control tags are exported with a `tag_` prefix. Service-tree ownership, resource identity, region, provider, instance type, and availability zone are included as target labels.

## Build

```bash
go test ./...
docker build --target server -t infraops-server:latest .
docker build --target agent -t infraops-agent:latest .
```

## Kubernetes

Create the MySQL connection secret, then apply the server and agent resources:

```bash
kubectl create secret generic infraops-server-secrets \
  --from-literal=mysql-dsn='user:password@tcp(mysql:3306)/infraops?charset=utf8&parseTime=True'
kubectl apply -f deploy/server.yaml
kubectl apply -f deploy/agent.yaml
```

Set `INFRAOPS_REGION` in `deploy/agent.yaml` to the region represented by that DaemonSet. Configuration files support environment-variable expansion.

ICMP defaults to unprivileged mode. If a cluster disables unprivileged ping and `options.privileged` is enabled, grant the Agent container `NET_RAW` explicitly instead of running it as a privileged container.
