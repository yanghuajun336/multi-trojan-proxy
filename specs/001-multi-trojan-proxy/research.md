# 技术研究：多Trojan客户端统一代理服务

**日期**: 2026-02-01  
**状态**: 完成  
**目的**: 为实现计划中的技术选型提供支撑

## 研究问题与决策

### 1. Trojan客户端库选择

**决策**: 使用 `github.com/p4gefau1t/trojan-go` 作为trojan客户端库

**理由**:
- 这是trojan协议的Go语言官方实现，活跃维护
- 提供了作为库使用的能力，不仅仅是CLI工具
- 支持标准trojan协议，兼容现有trojan服务器
- 纯Go实现，便于集成和交叉编译

**考虑的替代方案**:
- 直接调用trojan命令行工具 - 拒绝理由：进程管理复杂，性能开销大
- 自行实现trojan协议 - 拒绝理由：开发成本高，安全性难以保证
- 使用其他语言的trojan客户端 - 拒绝理由：跨语言调用复杂

**使用方式**:
```go
import (
    "github.com/p4gefau1t/trojan-go/proxy"
    "github.com/p4gefau1t/trojan-go/config"
    "github.com/p4gefau1t/trojan-go/tunnel"
)
```

---

### 2. HTTP代理服务器实现

**决策**: 使用Go标准库 `net/http` 实现HTTP/HTTPS代理服务器

**理由**:
- 标准库提供了完善的HTTP服务器支持，无需外部依赖
- `http.Server` 性能优异，支持并发连接
- 标准库自带超时控制、优雅关闭等功能
- 社区有大量HTTP代理实现参考

**核心实现模式**:
```go
// HTTP CONNECT 隧道（用于HTTPS）
func handleCONNECT(w http.ResponseWriter, r *http.Request) {
    // 1. Hijack连接获取底层TCP socket
    // 2. 通过trojan建立到目标的连接
    // 3. 双向转发数据
}

// 普通HTTP请求
func handleHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. 通过trojan转发请求
    // 2. 返回响应
}
```

**考虑的替代方案**:
- 使用第三方代理框架（如goproxy） - 拒绝理由：增加依赖，学习成本，不够灵活
- 直接使用net包从头实现 - 拒绝理由：重复造轮子，HTTP协议处理复杂

---

### 3. 连接池管理策略

**决策**: 为每个trojan节点维护独立的连接池，使用自定义连接池实现

**理由**:
- 重用连接避免频繁握手，降低延迟（符合SC-002性能要求）
- 每节点独立池便于健康管理和负载统计
- 自定义实现可精确控制连接生命周期和并发数

**连接池设计**:
```go
type ConnectionPool struct {
    node      *NodeConfig
    idle      chan *TrojanConn    // 空闲连接队列
    active    int                  // 活跃连接数
    maxIdle   int                  // 最大空闲连接（如5个）
    maxActive int                  // 最大活跃连接（如10个）
    timeout   time.Duration        // 连接超时（10秒）
}

// 获取连接：优先使用空闲连接，否则新建
func (p *ConnectionPool) Get() (*TrojanConn, error)

// 归还连接：放回池或关闭
func (p *ConnectionPool) Put(conn *TrojanConn)
```

**考虑的替代方案**:
- 使用sync.Pool - 拒绝理由：无法精确控制数量和生命周期
- 每次请求新建连接 - 拒绝理由：性能差，无法满足1.2倍延迟要求
- 全局共享连接池 - 拒绝理由：无法按节点隔离和统计

---

### 4. 负载均衡算法

**决策**: 实现多种负载均衡策略，默认使用加权轮询（Weighted Round Robin）

**理由**:
- 加权轮询简单高效，支持节点权重配置
- 可根据节点健康状态和性能动态调整
- 易于实现和测试，满足FR-008负载均衡需求

**支持的策略**:
1. **加权轮询（默认）**: 按节点权重循环分配
2. **最少连接**: 选择当前连接数最少的节点
3. **随机**: 随机选择健康节点（作为降级策略）

**选择器接口**:
```go
type NodeSelector interface {
    SelectNode(ctx context.Context) (*NodeConfig, error)
}

// 实现
type WeightedRoundRobinSelector struct {
    nodes   []*NodeConfig
    current int
    mutex   sync.Mutex
}
```

**考虑的替代方案**:
- 仅固定轮询 - 拒绝理由：无法处理节点性能差异
- 仅最少连接 - 拒绝理由：在节点性能相近时效果不明显
- 一致性哈希 - 拒绝理由：过度设计，本场景无会话保持需求

---

### 5. 健康检查机制

**决策**: 使用主动探测 + 被动检测的混合健康检查机制

**理由**:
- 满足FR-006要求的定期健康检查
- 主动探测确保及时发现节点故障（≤30秒间隔）
- 被动检测在实际请求失败时快速标记（<5秒故障转移）

**实现方案**:
```go
type HealthChecker struct {
    nodes        []*NodeConfig
    interval     time.Duration  // 30秒
    probeTimeout time.Duration  // 5秒
    
    // 健康状态
    statuses     map[string]*NodeStatus
}

type NodeStatus struct {
    Healthy      bool
    LastCheck    time.Time
    Latency      time.Duration
    FailCount    int
    SuccessRate  float64
}

// 主动探测：定期发起测试连接
func (hc *HealthChecker) ActiveProbe(node *NodeConfig)

// 被动检测：记录实际请求的成功/失败
func (hc *HealthChecker) RecordResult(node *NodeConfig, success bool, latency time.Duration)
```

**健康判定规则**:
- 连续3次探测失败 → 标记为不健康
- 成功率低于50% → 降低权重
- 延迟超过平均值2倍 → 降低权重
- 探测成功 → 恢复健康状态

**考虑的替代方案**:
- 仅主动探测 - 拒绝理由：无法快速响应突发故障
- 仅被动检测 - 拒绝理由：用户请求可能失败才发现问题
- 无健康检查 - 拒绝理由：无法满足FR-006和自动故障转移需求

---

### 6. 配置管理方案

**决策**: 使用YAML配置文件 + 文件监听实现动态重载

**理由**:
- YAML格式人类可读，易于编辑和维护
- 支持FR-010动态加载配置无需重启（60秒内生效）
- 文件监听机制成熟（使用fsnotify库）

**配置文件结构**:
```yaml
proxy:
  listen: "0.0.0.0:8080"
  timeout: 30s
  max_concurrent: 20

nodes:
  - name: "us-node-1"
    server: "us1.trojan.example.com"
    port: 443
    password: "password123"
    weight: 10
    enabled: true
    
  - name: "hk-node-1"
    server: "hk1.trojan.example.com"
    port: 443
    password: "password456"
    weight: 5
    enabled: true

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "info"
  file: "/var/log/proxy/proxy.log"
```

**配置重载流程**:
1. 监听配置文件变化（fsnotify）
2. 解析新配置并验证
3. 对比新旧配置，计算差异
4. 新增节点：创建连接池并加入选择器
5. 删除节点：优雅关闭连接池
6. 修改节点：更新配置，保持现有连接

**考虑的替代方案**:
- JSON配置 - 拒绝理由：不如YAML可读
- TOML配置 - 拒绝理由：Go生态YAML支持更好
- 环境变量 - 拒绝理由：不适合管理多节点配置
- 无动态重载 - 拒绝理由：不满足FR-010需求

---

### 7. 日志和可观测性

**决策**: 使用结构化日志 + 标准日志格式

**理由**:
- 满足FR-011日志记录需求
- 结构化日志便于解析和查询
- 标准格式便于与日志系统集成

**日志级别和内容**:
- **DEBUG**: 详细的请求路由、连接池操作
- **INFO**: 启动/关闭、配置重载、节点切换
- **WARN**: 节点失败、重试、性能告警
- **ERROR**: 严重错误、配置错误、无可用节点

**关键日志点**:
```go
// 节点切换
logger.Info("node switched", 
    "from", oldNode.Name,
    "to", newNode.Name,
    "reason", "health check failed")

// 请求失败
logger.Warn("request failed",
    "node", node.Name,
    "target", targetHost,
    "error", err,
    "retry", true)

// 配置重载
logger.Info("config reloaded",
    "added", len(added),
    "removed", len(removed),
    "modified", len(modified))
```

**考虑的替代方案**:
- 纯文本日志 - 拒绝理由：难以解析和查询
- 复杂日志系统（如ELK） - 拒绝理由：过度设计，增加部署复杂度
- 无日志 - 拒绝理由：不满足FR-011和可维护性要求

---

## 技术栈总结

### 核心依赖
```
github.com/p4gefau1t/trojan-go       # Trojan客户端库
gopkg.in/yaml.v3                     # YAML配置解析
github.com/fsnotify/fsnotify         # 文件监听（配置重载）
github.com/oschwald/geoip2-golang    # GeoIP查询库
```

### 标准库使用
```
net/http    # HTTP代理服务器
net         # TCP连接管理
sync        # 并发控制（互斥锁、等待组）
context     # 上下文传递和取消
time        # 超时和定时器
log         # 日志记录
```

### 开发工具
```
go test     # 单元测试
go build    # 编译打包
go mod      # 依赖管理
```

---

## 架构模式总结

**分层架构**:
```
┌─────────────────────────┐
│  HTTP Proxy Server      │ ← 接收客户端请求
├─────────────────────────┤
│  Router (Routing Rules) │ ← 决定DIRECT/PROXY/REJECT
├─────────────────────────┤
│  Node Selector          │ ← 负载均衡选择节点（仅PROXY）
├─────────────────────────┤
│  Connection Pool        │ ← 管理trojan连接
├─────────────────────────┤
│  Trojan Client Library  │ ← 与trojan服务器通信
└─────────────────────────┘

     并行服务
┌─────────────────────────┐
│  Health Checker         │ ← 定期健康检查
├─────────────────────────┤
│  Config Watcher         │ ← 监听配置变化
└─────────────────────────┘
```

**关键设计模式**:
- **池化模式**: 连接池管理
- **策略模式**: 可插拔的负载均衡算法、路由规则匹配
- **观察者模式**: 配置文件监听
- **单例模式**: 全局配置管理、GeoIP数据库
- **责任链模式**: 路由规则按顺序匹配

---

## 性能优化考虑

1. **连接复用**: 减少trojan握手开销
2. **并发处理**: 每个客户端请求独立goroutine
3. **零拷贝**: HTTP隧道使用io.Copy直接转发
4. **内存管理**: 限制连接数避免内存泄漏
5. **超时控制**: 各层级设置合理超时避免阻塞
6. **路由规则优化**:
   - 规则预编译和排序
   - 字典树加速域名后缀匹配
   - CIDR树加速IP段匹配
   - DNS和路由决策结果缓存
7. **GeoIP查询优化**: 内存映射数据库文件，避免频繁IO

---

## 风险和挑战

### 已识别风险
1. **Trojan-go库集成复杂度**: 可能需要深入源码理解API
   - 缓解措施: 阅读官方示例，参考社区实践
   
2. **并发安全**: 多goroutine访问共享状态
   - 缓解措施: 使用互斥锁保护临界区，设计无锁数据结构
   
3. **配置重载的原子性**: 避免重载过程中的不一致状态
   - 缓解措施: 先验证配置，再原子更新，旧资源延迟清理

### 未解决问题
- 无（所有技术选型和关键设计已确定）

---

## 下一步

研究完成，所有技术选型已确定。可以进入第1阶段：
- 生成数据模型定义（data-model.md）
- 生成配置文件合约（contracts/config-schema.yaml）
- 生成快速开始文档（quickstart.md）
