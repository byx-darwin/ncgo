#!/bin/bash
set -euo pipefail

# ============================================================
# ncgo rate-limit e2e 全流程验证脚本 — 6 场景覆盖
# ============================================================
# 前提条件：
#   1. Docker Desktop 已启动
#   2. ncgo 已编译（go build -o /tmp/ncgo .）
#   3. sqlc 已安装（go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest）
#   4. goose 已安装（go install github.com/pressly/goose/v3/cmd/goose@latest）
#   5. vegeta 已安装（go install github.com/tsenart/vegeta/v12@latest）
#
# 运行：bash scripts/e2e-test.sh
# ============================================================

NCGO="${NCGO:-/tmp/ncgo}"
PROJECT_DIR="/tmp/test-ratelimit"
REPORT_DIR="/tmp/test-ratelimit-reports"
GOOSE_DSN="postgres://postgres:postgres@localhost:5432/app?sslmode=disable"
GOOSE_DRIVER="pgx"

mkdir -p "$REPORT_DIR"

# ============================================================
# Helpers
# ============================================================

cleanup_service() {
  kill "$SERVICE_PID" 2>/dev/null || true
  sleep 1
  unset SERVICE_PID
}

wait_ready() {
  for i in $(seq 1 15); do
    if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "  服务启动超时，最后 20 行日志："
  tail -20 /tmp/test-ratelimit-service.log 2>/dev/null || true
  return 1
}

# Use Python to safely modify YAML config
set_rate_limit_config() {
  local conf_file="$PROJECT_DIR/conf/dev/conf.yaml"
  local source_type="$1"   # config | database | grpc
  local backend="$2"       # memory | redis

  python3 -c "
import re

with open('$conf_file', 'r') as f:
    lines = f.readlines()

in_rate_limit = False
for i, line in enumerate(lines):
    stripped = line.strip()
    if stripped == 'rate_limit:':
        in_rate_limit = True
    if in_rate_limit and re.match(r'^  enabled: false$', line):
        lines[i] = '  enabled: true\n'
        in_rate_limit = False
    if re.match(r'^    type: (config|grpc|database)', line):
        lines[i] = re.sub(r'^(\s+type: ).*', r'\1$source_type\n', line)
    if re.match(r'^  backend: (memory|redis)', line):
        lines[i] = re.sub(r'^(\s+backend: ).*', r'\1$backend\n', line)

with open('$conf_file', 'w') as f:
    f.writelines(lines)
"
}

inject_rate_limit_rules() {
  local conf_file="$PROJECT_DIR/conf/dev/conf.yaml"

  if grep -q 'max_requests: 10$' "$conf_file"; then
    return 0
  fi

  python3 -c "
with open('$conf_file', 'r') as f:
    content = f.read()

old = '    rules: []'
new = '''    rules:
      - match_kind: prefix
        path_pattern: /
        priority: 1
        rule:
          enabled: true
          key_by:
            - ip
          strategy: fixed_window
          window_seconds: 60
          max_requests: 10'''

content = content.replace(old, new, 1)

with open('$conf_file', 'w') as f:
    f.write(content)
"
}

run_e2e_for_scenario() {
  local scenario_name="$1"
  local report_md="$REPORT_DIR/${scenario_name}.md"
  local report_json="$REPORT_DIR/${scenario_name}.json"

  echo "  等待服务就绪..."
  if ! wait_ready; then
    echo "  FAIL: 服务未就绪"
    return 1
  fi
  echo "  服务已就绪"

  cd "$PROJECT_DIR"
  "$NCGO" test rate-limit e2e \
    --port 8080 \
    --rate 50 \
    --duration 10s \
    --paths /ping \
    --readiness-path /healthz \
    --cleanup false \
    --report "$report_md" || true

  "$NCGO" test rate-limit e2e \
    --port 8080 \
    --rate 50 \
    --duration 10s \
    --paths /ping \
    --readiness-path /healthz \
    --cleanup false \
    --report "$report_json" || true

  if [ -f "$report_md" ]; then
    status=$(grep '| Status |' "$report_md" | awk -F'|' '{print $3}' | xargs)
    printf "  结果: %s\n" "$status"
  fi
  echo "  报告: $report_md"
}

# ============================================================
# 步骤 1: 生成项目
# ============================================================
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

# ============================================================
# 步骤 2: 生成 SQLC + 依赖
# ============================================================
echo "=== 步骤 2: 生成 SQLC + 依赖 ==="
cd "$PROJECT_DIR/internal/db" && sqlc generate
cd "$PROJECT_DIR" && go mod tidy
echo "  SQLC 代码已生成，依赖已下载"

# ============================================================
# 步骤 3: 启动 PostgreSQL + Redis
# ============================================================
echo "=== 步骤 3: 启动 PostgreSQL + Redis ==="
cd "$PROJECT_DIR"
docker compose up -d postgres redis
echo "  等待数据库就绪..."
sleep 5

# ============================================================
# 步骤 4: 执行数据库迁移
# ============================================================
echo "=== 步骤 4: 执行数据库迁移 ==="
GOOSE_DBSTRING="$GOOSE_DSN" GOOSE_DRIVER="$GOOSE_DRIVER" make migrate-up
echo "  迁移完成"

# ============================================================
# 步骤 5: 注入限流规则到数据库（用于 database/grpc 场景）
# ============================================================
echo "=== 步骤 5: 注入限流规则到数据库 ==="
"$NCGO" test rate-limit seed \
  --root "$PROJECT_DIR" \
  --max-requests 10 \
  --window 60
echo "  规则已注入"

# ============================================================
# 步骤 6: 编译服务
# ============================================================
echo "=== 步骤 6: 编译服务 ==="
cd "$PROJECT_DIR" && GO_ENV=dev go build -o /tmp/test-ratelimit-bin .
echo "  编译完成"

# ============================================================
# 步骤 7: 运行 6 个场景
# ============================================================
echo ""
echo "========================================="
echo " 开始 6 场景 E2E 测试"
echo "========================================="

SCENARIOS=(
  "1-config-memory:config:memory"
  "2-config-redis:config:redis"
  "3-database-memory:database:memory"
  "4-database-redis:database:redis"
  "5-grpc-memory:grpc:memory"
  "6-grpc-redis:grpc:redis"
)

# Save original config for reset
cp "$PROJECT_DIR/conf/dev/conf.yaml" /tmp/test-ratelimit-conf-original.yaml

for entry in "${SCENARIOS[@]}"; do
  IFS=':' read -r name source_type backend <<< "$entry"
  echo ""
  echo "--- 场景 $name (source=$source_type, backend=$backend) ---"

  # 1. Kill old service
  cleanup_service

  # 2. Reset config to original, then apply scenario settings
  cp /tmp/test-ratelimit-conf-original.yaml "$PROJECT_DIR/conf/dev/conf.yaml"
  set_rate_limit_config "$source_type" "$backend"
  inject_rate_limit_rules

  # 3. Start service
  GO_ENV=dev /tmp/test-ratelimit-bin > /tmp/test-ratelimit-service.log 2>&1 &
  SERVICE_PID=$!

  # 4. Run e2e
  run_e2e_for_scenario "$name"

  # 5. Cleanup service before next scenario
  cleanup_service
done

# ============================================================
# 汇总
# ============================================================
echo ""
echo "========================================="
echo " 测试汇总"
echo "========================================="
echo ""

PASS_COUNT=0
FAIL_COUNT=0

for entry in "${SCENARIOS[@]}"; do
  IFS=':' read -r name _ _ <<< "$entry"
  report="$REPORT_DIR/${name}.md"
  if [ -f "$report" ]; then
    status=$(grep '| Status |' "$report" | awk -F'|' '{print $3}' | xargs)
    printf "  %-25s %s\n" "$name" "$status"
    if [ "$status" = "PASS" ]; then
      PASS_COUNT=$((PASS_COUNT + 1))
    else
      FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
  else
    printf "  %-25s NO REPORT\n" "$name"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
done

echo ""
echo "结果: $PASS_COUNT PASS / $((PASS_COUNT + FAIL_COUNT)) total"

# 清理
echo ""
echo "=== 清理 ==="
docker compose -f "$PROJECT_DIR/compose.yaml" stop postgres redis 2>/dev/null || true
echo "  服务已停止"
