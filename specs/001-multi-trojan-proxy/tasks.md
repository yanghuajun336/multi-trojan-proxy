# Tasks: 多Trojan客户端统一代理服务

**输入**: 设计文档来自 `/specs/001-multi-trojan-proxy/`  
**前置条件**: plan.md (必需), spec.md (必需), research.md, data-model.md, contracts/

**测试**: 本任务列表不包含测试任务，因为功能规格中未明确要求TDD方式开发。集成测试将在各用户场景完成后手动验证。

**组织方式**: 任务按用户场景分组，以支持独立实现和测试各个场景。

## 格式: `[ID] [P?] [Story] 描述`

- **[P]**: 可以并行执行（不同文件，无依赖）
- **[Story]**: 此任务属于哪个用户场景（如 US1, US2, US3）
- 描述中包含具体文件路径

## 路径约定

基于 plan.md 的项目结构：
- 源代码: `internal/`, `cmd/`, `pkg/`
- 配置: `configs/`
- 数据: `data/`
- 文档: `README.md`, `docs/`

---

## Phase 1: 项目初始化

**目的**: 创建Go项目基础结构和依赖配置

- [X] T001 创建项目目录结构（cmd/proxy, internal/{config,proxy,router,trojan,health}, pkg/logger, configs, data, tests）
- [X] T002 初始化Go模块 `go mod init github.com/yourname/proxy`
- [X] T003 [P] 添加核心依赖到go.mod（trojan-go, yaml.v3, fsnotify, geoip2-golang）
- [X] T004 [P] 创建 .gitignore 文件（忽略 config.yaml, *.log, data/*.mmdb, 可执行文件）
- [X] T005 [P] 创建 README.md 包含项目简介和快速开始链接
- [X] T006 [P] 复制 quickstart.md 到项目根目录的 docs/quickstart.md
- [X] T007 [P] 创建配置示例文件 configs/config.example.yaml（基于 contracts/config-schema.md）

---

## Phase 2: 基础设施（阻塞性前置条件）

**目的**: 实现所有用户场景依赖的核心基础设施

**⚠️ 关键**: 在此阶段完成前，任何用户场景都无法开始实现

- [X] T008 实现日志工具包 pkg/logger/logger.go（支持分级日志、文件输出、日志轮转）
- [X] T009 [P] 实现配置结构定义 internal/config/types.go（ProxyConfig, NodeConfig, RoutingRule等，基于data-model.md）
- [X] T010 实现配置文件加载 internal/config/loader.go（YAML解析、验证、错误处理）
- [X] T011 实现配置文件监听 internal/config/watcher.go（使用fsnotify，支持热重载）
- [X] T012 [P] 创建主程序入口 cmd/proxy/main.go（解析命令行参数、加载配置、启动服务）
- [X] T013 实现Trojan客户端封装 internal/trojan/client.go（连接trojan服务器、维护连接状态）

**检查点**: 基础设施就绪 - 用户场景实现可以并行开始

---

## Phase 3: 用户场景 1 - 统一代理入口访问国外网站 (优先级: P1) 🎯 MVP

**目标**: 实现基本的HTTP代理功能，支持单个或多个trojan节点，用户通过统一入口访问国外网站

**独立测试**: 使用 `curl -x http://localhost:8080 https://www.google.com` 验证代理功能，确认能成功访问国外网站

### 实现 用户场景 1

- [X] T014 [P] [US1] 实现HTTP代理服务器 internal/proxy/server.go（创建http.Server，监听配置的端口）
- [X] T015 [P] [US1] 实现HTTP请求处理器 internal/proxy/handler.go（处理HTTP和HTTPS CONNECT请求）
- [X] T016 [US1] 实现HTTP请求转发逻辑 internal/proxy/forwarder.go（通过trojan连接转发到目标服务器）
- [X] T017 [US1] 实现连接池基础结构 internal/trojan/pool.go（为每个节点维护连接池，支持获取/归还连接）
- [X] T018 [US1] 实现简单轮询节点选择器 internal/trojan/selector.go（基本的轮询算法选择可用节点）
- [X] T019 [US1] 在main.go中集成代理服务器启动逻辑
- [X] T020 [US1] 添加基本错误处理和日志记录（连接失败、转发错误等）

**检查点**: 此时用户场景 1 应完全可用并可独立测试 - 这就是MVP！

---

## Phase 4: 用户场景 2 - 自动故障转移与负载均衡 (优先级: P2)

**目标**: 实现节点健康检查、自动故障转移和负载均衡，提升服务可靠性

**独立测试**: 配置3个节点，停止其中一个节点的trojan服务，验证代理自动切换到其他节点；并发发送10个请求，验证负载分散

### 实现 用户场景 2

- [X] T021 [P] [US2] 实现节点状态结构 internal/health/status.go（NodeStatus定义，健康状态、延迟、统计信息）
- [X] T022 [P] [US2] 实现健康检查器 internal/health/checker.go（定期探测节点可用性和延迟）
- [X] T023 [US2] 实现主动健康探测 internal/health/probe.go（建立测试连接、测量延迟、记录结果）
- [X] T024 [US2] 实现被动健康检测 internal/health/passive.go（记录实际请求的成功/失败，快速标记异常）
- [X] T025 [US2] 增强节点选择器 internal/trojan/selector.go（加权轮询算法、排除不健康节点、支持多种策略）
- [X] T026 [US2] 实现故障转移逻辑 internal/proxy/failover.go（请求失败时重试其他节点）
- [X] T027 [US2] 在main.go中启动健康检查器
- [X] T028 [US2] 添加节点切换日志和统计信息更新

**检查点**: 用户场景 1 和 2 现在都应独立工作，服务具备高可用性

---

## Phase 5: 用户场景 4 - 智能路由规则过滤 (优先级: P2)

**目标**: 实现基于域名、IP、地理位置的路由规则，支持DIRECT/PROXY/REJECT动作

**独立测试**: 配置规则（baidu.com直连、google.com代理、192.168.0.0/16直连、CN地区直连），验证各规则正确匹配并路由

**注意**: 虽然此场景优先级也是P2，但它与US2无依赖，可以并行开发

### 实现 用户场景 4

- [X] T029 [P] [US4] 实现路由规则结构和类型 internal/router/types.go（RoutingRule定义、Action枚举）
- [X] T030 [P] [US4] 实现域名规则匹配器 internal/router/domain.go（DOMAIN和DOMAIN-SUFFIX匹配逻辑）
- [X] T031 [P] [US4] 实现IP规则匹配器 internal/router/ipcidr.go（IP-CIDR范围匹配，使用net.IPNet）
- [X] T032 [P] [US4] 实现GeoIP查询封装 internal/router/geoip.go（加载GeoLite2数据库、IP到国家代码查询）
- [X] T033 [US4] 实现路由引擎 internal/router/router.go（按顺序匹配规则、返回路由动作）
- [X] T034 [US4] 在配置加载中添加路由规则解析 internal/config/loader.go
- [X] T035 [US4] 在代理处理器中集成路由引擎 internal/proxy/handler.go（DIRECT请求直连、PROXY请求走trojan、REJECT请求拒绝）
- [X] T036 [US4] 实现DNS缓存 internal/router/dnscache.go（缓存域名解析结果，加速规则匹配）
- [X] T037 [US4] 添加规则匹配日志和统计（HitCount更新）

**检查点**: 用户场景 1, 2, 4 现在都应独立工作，服务支持智能路由

---

## Phase 6: 用户场景 3 - 多用户/多设备同时使用 (优先级: P3)

**目标**: 确保代理服务支持多客户端并发连接，处理并发场景

**独立测试**: 从3台不同设备同时连接代理，验证所有设备都能正常使用

**注意**: 此功能大部分已由Go的并发特性和前面的实现支持，主要是验证和优化

### 实现 用户场景 3

- [X] T038 [US3] 验证并发连接处理 internal/proxy/server.go（确认每个连接独立goroutine处理）
- [X] T039 [US3] 实现并发连接数限制 internal/proxy/server.go（基于MaxConcurrent配置，防止过载）
- [X] T040 [US3] 实现客户端连接跟踪 internal/proxy/connections.go（记录活跃连接、统计信息）
- [X] T041 [US3] 添加连接级别的超时控制 internal/proxy/handler.go（避免连接长时间占用）
- [X] T042 [US3] 优化连接池并发安全 internal/trojan/pool.go（使用互斥锁保护共享状态）

**检查点**: 所有主要用户场景 (1, 2, 3, 4) 现在都应独立工作

---

## Phase 7: 用户场景 5 - 节点健康监控与配置管理 (优先级: P4)

**目标**: 提供节点状态查询能力，支持配置动态重载

**独立测试**: 修改config.yaml添加新节点，验证60秒内自动加载；查看日志确认节点状态信息

### 实现 用户场景 5

- [X] T043 [P] [US5] 实现节点状态查询接口 internal/health/query.go（提供节点健康状态、延迟、成功率查询）
- [X] T044 [US5] 在配置监听器中实现配置差异计算 internal/config/watcher.go（对比新旧配置，识别新增/删除/修改）
- [X] T045 [US5] 实现节点动态添加逻辑 internal/trojan/pool.go（为新节点创建连接池）
- [X] T046 [US5] 实现节点动态移除逻辑 internal/trojan/pool.go（优雅关闭连接池，停止使用节点）
- [X] T047 [US5] 实现节点配置更新逻辑 internal/trojan/pool.go（更新现有节点配置）
- [X] T048 [US5] 添加配置重载完整日志（记录新增、删除、修改的节点）

**检查点**: 所有用户场景都已实现

---

## Phase 8: 完善与跨场景改进

**目的**: 完善文档、优化性能、增强可维护性

- [X] T049 [P] 创建完整的README.md（项目介绍、功能特性、安装步骤、配置说明、使用示例）
- [ ] T050 [P] 创建详细的配置文件文档 docs/configuration.md（基于 contracts/config-schema.md）
- [X] T051 [P] 添加代码注释和GoDoc文档（所有公开函数和类型）
- [X] T052 实现优雅关闭 cmd/proxy/main.go（监听信号、优雅停止服务、关闭连接）
- [ ] T053 [P] 性能优化：路由规则预编译和索引优化 internal/router/router.go
- [ ] T054 [P] 性能优化：实现规则匹配结果缓存 internal/router/cache.go（LRU cache）
- [ ] T055 [P] 安全加固：配置文件权限检查 internal/config/loader.go（警告不安全权限）
- [ ] T056 运行 quickstart.md 中的所有验证步骤，确保文档准确
- [X] T057 [P] 创建systemd服务文件示例 configs/proxy.service
- [X] T058 [P] 添加Makefile（build, install, clean等目标）
- [ ] T059 代码审查和重构（消除重复代码、改进命名、统一错误处理）
- [ ] T060 [P] 下载并配置GeoLite2数据库到 data/GeoLite2-Country.mmdb

---

## 依赖关系与执行顺序

### 阶段依赖

- **项目初始化 (Phase 1)**: 无依赖 - 可立即开始
- **基础设施 (Phase 2)**: 依赖 Phase 1 完成 - 阻塞所有用户场景
- **用户场景 (Phase 3-7)**: 都依赖 Phase 2 完成
  - 用户场景可以并行开发（如果有多人团队）
  - 或按优先级顺序开发 (P1 → P2 → P3 → P4)
- **完善 (Phase 8)**: 依赖所需的用户场景完成

### 用户场景依赖关系

- **用户场景 1 (P1)**: Phase 2 完成后即可开始 - 无其他场景依赖
- **用户场景 2 (P2)**: Phase 2 完成后即可开始 - 依赖US1的连接池和选择器，建议在US1后开发
- **用户场景 4 (P2)**: Phase 2 完成后即可开始 - 与US2无依赖，可并行开发
- **用户场景 3 (P3)**: Phase 2 完成后即可开始 - 主要是验证和优化现有并发能力
- **用户场景 5 (P4)**: 依赖US2的健康检查 - 建议在US2后开发

### 每个用户场景内部

- 基础组件优先（如types, client, pool）
- 核心逻辑次之（如router, selector, checker）
- 集成和优化最后（如failover, cache）

### 并行机会

- Phase 1 中所有标记 [P] 的任务可以并行
- Phase 2 中标记 [P] 的任务可以并行（在同一阶段内）
- Phase 2 完成后，US1, US4可以并行开发
- US2 和 US4 可以并行开发（不同模块）
- Phase 8 中所有标记 [P] 的任务可以并行

---

## 并行示例: 用户场景 1

```bash
# 同时启动US1的多个独立组件:
Task: "实现HTTP代理服务器 internal/proxy/server.go"
Task: "实现HTTP请求处理器 internal/proxy/handler.go"
# 等待这些完成后再做集成
```

---

## 并行示例: 用户场景 4

```bash
# 同时实现各类规则匹配器:
Task: "实现域名规则匹配器 internal/router/domain.go"
Task: "实现IP规则匹配器 internal/router/ipcidr.go"  
Task: "实现GeoIP查询封装 internal/router/geoip.go"
# 然后集成到router.go
```

---

## 实施策略

### MVP优先 (仅用户场景 1)

1. 完成 Phase 1: 项目初始化
2. 完成 Phase 2: 基础设施 (关键 - 阻塞所有场景)
3. 完成 Phase 3: 用户场景 1
4. **停止并验证**: 独立测试用户场景 1
5. 如果就绪，可部署/演示

### 增量交付

1. 完成 初始化 + 基础设施 → 基础就绪
2. 添加 用户场景 1 → 独立测试 → 部署/演示 (MVP!)
3. 添加 用户场景 2 → 独立测试 → 部署/演示（高可用版本）
4. 添加 用户场景 4 → 独立测试 → 部署/演示（智能路由版本）
5. 添加 用户场景 3 → 独立测试 → 部署/演示（多用户版本）
6. 添加 用户场景 5 → 独立测试 → 部署/演示（完整版本）
7. 每个场景都增加价值而不破坏之前的场景

### 并行团队策略

如果有多个开发者:

1. 团队一起完成 初始化 + 基础设施
2. 基础设施完成后:
   - 开发者 A: 用户场景 1
   - 开发者 B: 用户场景 4 (与US1并行)
   - 开发者 C: 准备Phase 8的文档和配置
3. US1完成后:
   - 开发者 A: 用户场景 2（基于US1）
   - 开发者 B: 继续US4或开始US3
4. 场景独立完成并集成

---

## 任务统计

- **总任务数**: 60
- **Phase 1 (初始化)**: 7 任务
- **Phase 2 (基础设施)**: 6 任务 ⚠️ 阻塞
- **Phase 3 (US1-P1)**: 7 任务 🎯 MVP
- **Phase 4 (US2-P2)**: 8 任务
- **Phase 5 (US4-P2)**: 9 任务
- **Phase 6 (US3-P3)**: 5 任务
- **Phase 7 (US5-P4)**: 6 任务
- **Phase 8 (完善)**: 12 任务

- **并行任务**: 25个标记[P]的任务可以并行执行

---

## 备注

- [P] 任务 = 不同文件，无依赖关系
- [Story] 标签将任务映射到具体用户场景，便于追溯
- 每个用户场景应该可以独立完成和测试
- 在实现前确保理解data-model.md和contracts/
- 每个任务或逻辑组完成后提交代码
- 在任何检查点停止以独立验证场景
- 避免：模糊任务、同文件冲突、破坏独立性的跨场景依赖
