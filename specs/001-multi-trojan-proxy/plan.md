# 实现计划：多Trojan客户端统一代理服务

**分支**: `001-multi-trojan-proxy` | **日期**: 2026-02-01 | **规格**: [spec.md](./spec.md)
**输入**: 功能规格来自 `/specs/001-multi-trojan-proxy/spec.md`

**说明**: 此模板由 `/speckit.plan` 命令填充。

## 概要

构建一个统一的HTTP代理服务，内部管理多个trojan客户端连接，对外暴露单一代理入口(proxy.domain.com:port)。系统实现自动故障转移和负载均衡，支持多客户端并发访问，通过健康检查机制确保高可用性。技术方案采用Go语言开发高性能代理服务器，使用连接池管理多个trojan客户端，实现智能路由和故障检测。

## 技术上下文

**语言/版本**: Go 1.21+  
**主要依赖**: 
- trojan-go (trojan客户端库)
- 标准库 net/http (HTTP代理服务器)
- 标准库 sync (并发控制)
- yaml.v3 (配置文件解析)

**存储**: 
- 配置文件: YAML格式存储trojan节点配置
- 运行时状态: 内存中维护节点健康状态和统计信息
- 日志: 文件系统存储日志

**测试**: 
- 单元测试: Go testing 标准库
- 集成测试: 模拟trojan服务器进行端到端测试
- 合约测试: 验证HTTP代理协议兼容性

**目标平台**: Linux服务器 (Ubuntu 20.04+)  

**项目类型**: 单体应用（single）- HTTP代理服务器

**性能目标**: 
- 支持至少10个并发客户端连接
- 代理延迟增加不超过单独使用trojan的1.2倍
- 故障转移时间小于5秒
- 节点健康检查间隔不超过30秒

**约束条件**: 
- 内存使用: 正常运行不超过100MB
- CPU使用: 空闲时<5%, 高负载时<50%
- 连接超时: HTTP请求超时30秒，trojan连接超时10秒
- 兼容性: 支持标准HTTP/HTTPS CONNECT代理协议

**规模/范围**: 
- 支持配置和管理20+个trojan节点
- 同时维护10+个活跃客户端连接
- 单个实例服务10-20人团队使用

## 宪法检查

*门控: 必须在第0阶段研究前通过。在第1阶段设计后重新检查。*

**注意**: 当前项目的constitution.md尚未填写具体原则。假设采用以下通用软件工程最佳实践：

### 检查项

- [x] **模块化设计**: 代码按功能模块组织（代理服务器、节点管理、健康检查、配置管理）
- [x] **可测试性**: 核心逻辑与外部依赖解耦，便于单元测试
- [x] **错误处理**: 完善的错误处理和日志记录
- [x] **配置管理**: 使用配置文件，避免硬编码
- [x] **文档化**: 提供README、配置示例、API文档

### 第1阶段后重新检查

_(第1阶段设计完成后填写)_

## Project Structure

### 文档 (本功能)

```text
specs/001-multi-trojan-proxy/
├── plan.md              # 本文件 (/speckit.plan 命令输出)
├── research.md          # 第0阶段输出 (/speckit.plan 命令)
├── data-model.md        # 第1阶段输出 (/speckit.plan 命令)
├── quickstart.md        # 第1阶段输出 (/speckit.plan 命令)
├── contracts/           # 第1阶段输出 (/speckit.plan 命令)
│   └── config-schema.yaml  # 配置文件结构定义
└── tasks.md             # 第2阶段输出 (/speckit.tasks 命令 - 不由 /speckit.plan 创建)
```

### 源代码 (代码仓库根目录)

```text
# 单体项目结构
proxy/                   # 项目根目录
├── cmd/
│   └── proxy/          # 主程序入口
│       └── main.go
├── internal/           # 内部包（不导出）
│   ├── config/        # 配置管理
│   │   ├── config.go  # 配置结构和加载
│   │   └── watcher.go # 配置文件监听
│   ├── proxy/         # HTTP代理服务器
│   │   ├── server.go  # 代理服务器主逻辑
│   │   └── handler.go # HTTP请求处理
│   ├── router/        # 路由规则引擎
│   │   ├── router.go  # 路由决策主逻辑
│   │   ├── rule.go    # 规则匹配实现
│   │   └── geoip.go   # GeoIP查询封装
│   ├── trojan/        # Trojan节点管理
│   │   ├── client.go  # Trojan客户端封装
│   │   ├── pool.go    # 连接池管理
│   │   └── selector.go # 节点选择器（负载均衡）
│   └── health/        # 健康检查
│       ├── checker.go # 健康检查逻辑
│       └── monitor.go # 节点状态监控
├── pkg/               # 可导出的公共包
│   └── logger/        # 日志工具
├── configs/           # 配置文件示例
│   └── config.example.yaml
├── data/              # 数据文件目录
│   └── GeoLite2-Country.mmdb  # GeoIP数据库（可选）
├── tests/             # 测试
│   ├── integration/   # 集成测试
│   │   └── proxy_test.go
│   └── unit/          # 单元测试 (也可以和源码放一起)
├── go.mod
├── go.sum
└── README.md
```

**结构决策**: 
- 采用Go标准项目布局
- 使用`internal/`确保包不被外部导入
- 使用`cmd/`存放可执行程序入口
- 新增`internal/router/`模块处理路由规则
- 新增`data/`目录存放GeoIP数据库等数据文件
- 测试文件可以和源码放在一起（_test.go），也可以集中在tests/目录
- 配置文件示例放在configs/目录，实际运行时配置由用户提供

## 复杂度追踪

> **仅在宪法检查有必须说明的违规时填写**

本项目未检测到需要说明的复杂度违规。架构设计遵循简单性原则：
- 单体应用，无多项目复杂性
- 标准Go项目结构，无过度抽象
- 直接使用trojan客户端库，无额外中间层
- 配置驱动，避免硬编码
