# K8s 基础设施生成设计

- 状态：Draft
- 关联：`specs/010-v1.0-plan.md` Phase 3 / P2-1

## 1. 目标

`ncgo add infra k8s`：为已有 Dockerfile 的 ncgo 项目生成 Kubernetes 部署文件。

## 2. 命令

```bash
ncgo add infra k8s --root .
ncgo add infra k8s --root . --dry-run
ncgo add infra k8s --root . --output json
```

## 3. 生成内容

```
<project>/
  deploy/
    k8s/
      base/
        deployment.yaml     ← Deployment + 健康检查
        service.yaml        ← ClusterIP Service
        configmap.yaml      ← 从 conf/ 生成（可选）
        kustomization.yaml  ← base kustomization
      overlays/
        dev/
          kustomization.yaml   ← dev overlay（副本数、镜像 tag）
        prod/
          kustomization.yaml   ← prod overlay（副本数、资源限制、HPA）
```

### 3.1 deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.ServiceName}}
  labels:
    app: {{.ServiceName}}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{.ServiceName}}
  template:
    metadata:
      labels:
        app: {{.ServiceName}}
    spec:
      containers:
      - name: {{.ServiceName}}
        image: {{.Module}}:latest
        ports:
        - containerPort: {{.Port}}
        envFrom:
        - configMapRef:
            name: {{.ServiceName}}-config
        livenessProbe:
          httpGet:
            path: /health
            port: {{.Port}}
        readinessProbe:
          httpGet:
            path: /health
            port: {{.Port}}
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
```

### 3.2 端口推断

| Kind | 默认端口 |
|------|----------|
| hertz | 8080 |
| kitex | 8888 |

### 3.3 kustomization.yaml (dev)

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../base
replicas:
- name: {{.ServiceName}}
  count: 1
images:
- name: {{.Module}}
  newTag: dev
```

### 3.4 kustomization.yaml (prod)

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../base
replicas:
- name: {{.ServiceName}}
  count: 3
images:
- name: {{.Module}}
  newTag: v1.0.0
patches:
- target:
    kind: Deployment
    name: {{.ServiceName}}
  patch: |-
    - op: replace
      path: /spec/template/spec/containers/0/resources
      value:
        requests:
          memory: "256Mi"
          cpu: "500m"
        limits:
          memory: "1Gi"
          cpu: "2000m"
```

## 4. 模板系统

模板放 `internal/assets/_data/optional/k8s/`：

```
internal/assets/_data/optional/k8s/
  base/
    deployment.yaml.tmpl
    service.yaml.tmpl
    configmap.yaml.tmpl
    kustomization.yaml.tmpl
  overlays/
    dev/
      kustomization.yaml
    prod/
      kustomization.yaml
      hpa.yaml.tmpl
```

## 5. Infra 集成

如果 manifest 中已有 infra 配置，自动补充：

| Infra | K8s 补充 |
|-------|----------|
| redis | 加 Redis Service + 环境变量 |
| kafka | 加 Kafka Service（PLAINTEXT 端口） |
| es | 加 Elasticsearch Service |
| postgres | 加 Postgres Service + Secret |
| etcd | 加 Etcd Service |

## 6. 范围边界

v1.0 先做 Kustomize overlay 模式（最通用，不绑定具体发行版）。不做 Helm chart（等社区反馈）。不做 Istio/服务网格注入。

## 7. 测试

- Golden test：对 hertz-default + kitex-default 的 golden testdata 生成 k8s 文件
- `--dry-run` 输出验证
- 已有 k8s 文件的冲突处理
