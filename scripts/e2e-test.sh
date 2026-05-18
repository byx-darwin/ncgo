#!/bin/bash
set -euo pipefail

# ============================================================
# ncgo rate-limit e2e 全流程验证脚本
# ============================================================
# 前提条件：
#   1. Docker Desktop 已启动
#   2. ncgo 已编译（go build -o /tmp/ncgo .）
#   3. sqlc 已安装（go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest）
#
# 运行：bash scripts/e2e-test.sh
# ============================================================

NCGO="${NCGO:-/tmp/ncgo}"
PROJECT_DIR="/tmp/test-ratelimit"
REPORT_MD="/tmp/test-ratelimit/e2e-report.md"
REPORT_JSON="/tmp/test-ratelimit/e2e-report.json"

echo "=== 步骤 1: 生成项目 ==="
rm -rf "$PROJECT_DIR"
"$NCGO" new test-ratelimit \
  --mode mono \
  --kind hertz \
  --module github.com/test/ratelimit-demo \
  --db postgres \
  --infra redis \
  --dir "$PROJECT_DIR"
echo "  项目生成于 $PROJECT_DIR"

echo "=== 步骤 2: 生成 SQLC + 依赖 ==="
cd "$PROJECT_DIR/internal/db" && sqlc generate
cd "$PROJECT_DIR" && go mod tidy
echo "  SQLC 代码已生成，依赖已下载"

echo "=== 步骤 3: 配置限流 ==="
# 开启限流 + 添加严格规则 (10 req/60s)
sed -i '' '198s/enabled: false/enabled: true/' "$PROJECT_DIR/conf/dev/conf.yaml"
sed -i '' '251s/rules: \[\]/rules:\n      - match_kind: prefix\n        path_pattern: \/\n        priority: 1\n        rule:\n          enabled: true\n          key_by:\n            - ip\n          strategy: fixed_window\n          window_seconds: 60\n          max_requests: 10/' "$PROJECT_DIR/conf/dev/conf.yaml"
echo "  限流已启用 (10 req/60s)"

echo "=== 步骤 4: 启动 PostgreSQL + Redis ==="
cd "$PROJECT_DIR"
docker compose up -d postgres redis
echo "  等待数据库就绪..."
sleep 5

echo "=== 步骤 5: 执行数据库迁移 ==="
GOOSE_DBSTRING="postgres://postgres:postgres@localhost:5432/app?sslmode=disable" GOOSE_DRIVER=postgres make migrate-up
echo "  迁移完成"

echo "=== 步骤 6: 启动服务 ==="
GO_ENV=dev go build -o /tmp/test-ratelimit-bin .
GO_ENV=dev /tmp/test-ratelimit-bin > /tmp/test-ratelimit-service.log 2>&1 &
SERVICE_PID=$!
echo "  服务 PID=$SERVICE_PID"

# 健康检查
echo "  等待服务就绪..."
for i in $(seq 1 15); do
  if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
    echo "  服务已就绪"
    break
  fi
  if [ "$i" -eq 15 ]; then
    echo "  服务启动超时"
    cat /tmp/test-ratelimit-service.log
    kill "$SERVICE_PID" 2>/dev/null || true
    docker compose down
    exit 1
  fi
  sleep 2
done

echo "=== 步骤 7: 运行 E2E 测试 ==="
cd "$PROJECT_DIR"
"$NCGO" test rate-limit e2e \
  --port 8080 \
  --rate 50 \
  --duration 10s \
  --paths /ping \
  --readiness-path /healthz \
  --cleanup false \
  --report "$REPORT_MD"
echo ""
echo "=== 步骤 8: 生成 JSON 报告 ==="
"$NCGO" test rate-limit e2e \
  --port 8080 \
  --rate 50 \
  --duration 10s \
  --paths /ping \
  --readiness-path /healthz \
  --cleanup false \
  --report "$REPORT_JSON"

echo ""
echo "=== 测试完成 ==="
echo "Markdown 报告: $REPORT_MD"
echo "JSON 报告:     $REPORT_JSON"

# 清理
echo "=== 清理 ==="
kill "$SERVICE_PID" 2>/dev/null || true
docker compose stop postgres redis
echo "  服务已停止"
