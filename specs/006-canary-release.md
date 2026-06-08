# Hertz / Kitex Canary Release Design

## 1. Philosophy

Canary release logic should NOT be written primarily in Hertz handlers, Kitex handlers, or usecase business logic.

Recommended separation into four layers:

1. **Release layer**: CI/CD, Kubernetes, Argo Rollouts, manual/auto rollback.
2. **Traffic layer**: Ingress/Gateway/Service Mesh, Kitex client selector.
3. **Registry, Discovery & Config layer**: Nacos / Polaris for service registration, discovery, instance metadata, and dynamic canary rules.
4. **Observability layer**: Logs, metrics, traces aggregated by version and traffic lane.

`ncgo` provides unified conventions, templates, and helpers — not a full release platform.

## 2. Goals

Support unified canary release for Hertz and Kitex services:

- Stable / canary release tracks.
- Traffic splitting by weight, header, user, or tenant.
- Nacos and Polaris as registries.
- Nacos and Polaris as service discovery sources.
- Nacos and Polaris as config centers.
- Dynamic canary rule adjustment without restart.
- Fast rollback.
- Observability and alerting by release metadata.

## 3. Non-Goals (v1)

- Full release platform.
- Auto-modify user Kubernetes YAML.
- Auto-create Nacos / Polaris rules.
- Generate business canary branching in usecases.
- Multi-experiment platform or complex A/B testing DSL.
- Auto-rollback executor.

## 4. Core Concepts

### 4.1 Release Tracks

| Track | Meaning |
|---|---|
| `stable` | Current stable version, default carries all or most traffic |
| `canary` | New version, small-traffic validation |

### 4.2 Service Instance Metadata

Each service instance carries:
- `release.track`: `stable` or `canary`
- `release.version`: service version string
- `release.commit`: build commit SHA
- `release.build_time`: build timestamp

### 4.3 Traffic Context

Traffic carries lane selection hints via:
- HTTP headers: `X-Traffic-Lane`, `X-User-ID`, `X-Tenant-ID`
- Kitex metadata: `traffic.lane`, `traffic.user_id`, `traffic.tenant_id`

### 4.4 Canary Rules

- Priority-ordered rule list.
- Match on: header, cookie, user, tenant, region.
- Actions: route to stable or canary.
- Default fallback: `fallback=stable` or `fallback=fail_fast`.
- Weights control traffic percentage to each pool.

## 5. Template Benefits

`ncgo add infra release_canary` generates:

- `internal/base/release/canary.go`: core abstractions
- `internal/base/release/hertz.go`: Hertz header adapter
- `internal/base/release/kitex.go`: Kitex metadata adapter

Optional `--wire` integrates middleware into server/client wiring.
