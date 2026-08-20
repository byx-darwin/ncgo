# kitex-client Subcommand Design Spec

**Date:** 2026-08-20
**Issue:** [#79](https://github.com/byx-darwin/ncgo/issues/79)
**Status:** Draft

## Overview

添加 `ncgo add kitex-client` 子命令，使得 BFF 服务能够生成调用 RPC 服务所需的 Kitex 客户端代码。

**Bar:** 可工作的 CLI 命令 + 生成的客户端代码能编译 + 文档完整

## Problem Analysis

### Root Cause

当前 `ncgo add bff` 只生成 Hertz 相关代码，不会生成 Kitex 客户端代码。BFF 服务需要调用 RPC 服务时，缺少：
- Kitex 客户端包装器（pkg/client/<name>/）
- 从 proto 文件生成的 kitex_gen 类型
- 配置和连接管理代码

### Impact

- BFF 代码编译失败（引用不存在的 Kitex 客户端）
- 用户需要手动编写客户端代码
- 违反 ncgo 的"脚手架自动化"理念

## Scope

### 实现内容

| 类别 | 内容 |
|------|------|
| **CLI 命令** | `ncgo add kitex-client <name> --service <service> --idl <path>` |
| **代码生成** | pkg/client/<name>/client.go + config.go |
| **类型生成** | kitex_gen/（从 proto 文件） |
| **依赖管理** | 更新 go.mod |
| **文档** | 命令帮助 + README 更新 |

### 不在 Scope

- **修改 ncgo add bff** — 不改变现有 bff 命令
- **自动检测依赖** — 不实现自动检测 IDL import
- **多语言支持** — 只支持 Go

## Architecture

### 命令设计

```bash
ncgo add kitex-client <name> --service <rpc-service-name> --idl <proto-path>
```

**参数：**
- `<name>` — 客户端名称（如 rbac, rulecenter）
- `--service` — RPC 服务名称（用于生成客户端）
- `--idl` — proto 文件路径（用于生成 kitex_gen）
- `--root` — 项目根目录（默认 .）
- `--force` — 覆盖现有文件
- `--dry-run` — 预览不写入

**示例：**
```bash
cd services/admin

# 添加 rbac 客户端
ncgo add kitex-client rbac \
  --service rbac-rpc \
  --idl ../../rbac/idl/rbac.proto

# 添加 rule-center 客户端
ncgo add kitex-client rulecenter \
  --service rule-rpc \
  --idl ../../rule/idl/rule_center.proto
```

### 生成的文件

```
services/admin/
├── pkg/client/
│   ├── rbac/
│   │   ├── client.go          # Kitex 客户端包装器
│   │   └── config.go          # 客户端配置
│   └── rulecenter/
│       ├── client.go
│       └── config.go
└── kitex_gen/
    ├── api/rbac/v1/           # 从 proto 生成的类型
    │   ├── auth.pb.go
    │   ├── auth_service.pb.go
    │   └── ...
    └── api/ratelimit/v1/
        └── ...
```

### 代码模板

**client.go 模板：**
```go
package {{.ClientName}}

import (
    "context"
    "{{.Module}}/kitex_gen/{{.PackagePath}}"
    "github.com/cloudwego/kitex/client"
)

type Client struct {
    client {{.ServiceName}}.Client
}

func New(addr string) (*Client, error) {
    c, err := {{.ServiceName}}.NewClient(addr, client.WithHostPorts(addr))
    if err != nil {
        return nil, err
    }
    return &Client{client: c}, nil
}

// 生成服务方法代理
{{range .Methods}}
func (c *Client) {{.Name}}(ctx context.Context, req *{{.RequestType}}) (*{{.ResponseType}}, error) {
    return c.client.{{.Name}}(ctx, req)
}
{{end}}
```

**config.go 模板：**
```go
package {{.ClientName}}

type Config struct {
    Address string `yaml:"address"`
}

func LoadConfig() (*Config, error) {
    // 从配置加载
    return &Config{Address: "localhost:{{.Port}}"}, nil
}
```

## Implementation Plan

### Task 1: 创建 CLI 命令

**Files:**
- Create: `internal/cli/add_kitex_client.go`

**Steps:**
1. 定义 addKitexClientOptions struct
2. 创建 newAddKitexClientCmd() 函数
3. 实现 runAddKitexClient() 函数
4. 在 add.go 中注册命令

### Task 2: 创建 scaffold 包

**Files:**
- Create: `internal/scaffold/kitexclient/kitexclient.go`
- Create: `internal/scaffold/kitexclient/templates.go`

**Steps:**
1. 定义 Options struct 和 Result struct
2. 实现 Add() 函数（核心逻辑）
3. 实现模板渲染
4. 实现文件写入

### Task 3: 添加代码模板

**Files:**
- Create: `internal/scaffold/kitexclient/templates/client.go.tpl`
- Create: `internal/scaffold/kitexclient/templates/config.go.tpl`

**Steps:**
1. 创建 client.go 模板
2. 创建 config.go 模板
3. 添加模板数据解析逻辑

### Task 4: 集成 kitex 代码生成

**Files:**
- Modify: `internal/scaffold/kitexclient/kitexclient.go`

**Steps:**
1. 调用 kitex 命令生成 kitex_gen
2. 处理 IDL 路径和包名
3. 验证生成的代码

### Task 5: 测试

**Files:**
- Create: `internal/cli/add_kitex_client_test.go`
- Create: `internal/scaffold/kitexclient/kitexclient_test.go`

**Steps:**
1. 单元测试 CLI 命令
2. 单元测试 scaffold 逻辑
3. 集成测试（生成代码并编译）

### Task 6: 文档

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`

**Steps:**
1. 添加命令说明
2. 添加使用示例
3. 更新文档

## Acceptance Criteria

- [ ] `ncgo add kitex-client` 命令可执行
- [ ] 支持 `--service` 和 `--idl` 参数
- [ ] 生成 pkg/client/<name>/client.go 和 config.go
- [ ] 生成 kitex_gen/ 类型（调用 kitex 命令）
- [ ] 更新 go.mod 依赖
- [ ] 生成的代码能编译通过
- [ ] 单元测试覆盖
- [ ] 文档完整

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| kitex 命令不可用 | 检查 kitex 是否安装，提供安装指引 |
| IDL 路径错误 | 验证 IDL 文件存在，解析 proto 包名 |
| 生成的代码编译失败 | 提供示例测试，验证生成逻辑 |
| 依赖冲突 | 使用 go mod tidy 清理依赖 |

## Related

- Issue #79 — kitex-client subcommand
- ncgo-templates#19 — micro-admin 模板重新设计
- ncgo-templates#13 — micro-admin 模板初始实现
