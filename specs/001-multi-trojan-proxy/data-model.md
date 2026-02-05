# 数据模型：多Trojan客户端统一代理服务

**日期**: 2026-02-01  
**状态**: 设计  
**来源**: 从功能规格(spec.md)提取的实体定义

## 核心实体

### 1. ProxyConfig - 代理服务配置

代表整个代理服务的全局配置。

**字段**:
```go
type ProxyConfig struct {
    Listen         string        // 监听地址，如 "0.0.0.0:8080"
    Timeout        time.Duration // HTTP请求超时时间，默认30秒
    MaxConcurrent  int           // 最大并发连接数，默认20
    HealthCheck    HealthCheckConfig
    Logging        LoggingConfig
    Nodes          []NodeConfig  // Trojan节点列表
}
```

**验证规则**:
- `Listen` 必须是有效的地址:端口格式
- `Timeout` 必须 > 0，建议范围 10-60秒
- `MaxConcurrent` 必须 > 0，建议 10-100
- `Nodes` 至少包含1个节点

**状态转换**: 静态配置，仅通过配置文件重载更新

---

### 2. NodeConfig - Trojan节点配置

代表单个trojan服务器的连接配置。

**字段**:
```go
type NodeConfig struct {
    Name     string  // 节点名称/标识，唯一
    Server   string  // Trojan服务器地址
    Port     int     // Trojan服务器端口，通常443
    Password string  // Trojan认证密码
    Weight   int     // 负载均衡权重，1-100，默认10
    Enabled  bool    // 是否启用此节点，默认true
}
```

**验证规则**:
- `Name` 必须非空且在配置中唯一
- `Server` 必须是有效的域名或IP地址
- `Port` 必须在 1-65535 范围内
- `Password` 必须非空
- `Weight` 必须 > 0，建议 1-100
- `Enabled` 为false时节点不参与负载均衡

**关系**:
- 被 `ProxyConfig` 包含（一对多）
- 关联一个 `ConnectionPool`（一对一）
- 关联一个 `NodeStatus`（一对一）

**状态转换**:
```
配置中 → 启用(Enabled=true) → 参与负载均衡
       → 禁用(Enabled=false) → 不参与负载均衡
```

---

### 3. NodeStatus - 节点健康状态

代表运行时节点的健康和性能统计信息。

**字段**:
```go
type NodeStatus struct {
    NodeName      string        // 关联的节点名称
    Healthy       bool          // 当前健康状态
    LastCheck     time.Time     // 最后一次健康检查时间
    LastSuccess   time.Time     // 最后一次成功时间
    Latency       time.Duration // 平均延迟
    FailCount     int           // 连续失败次数
    TotalRequests int64         // 总请求数
    FailedRequests int64        // 失败请求数
    SuccessRate   float64       // 成功率 (0-1)
    ActiveConns   int           // 当前活跃连接数
}
```

**计算字段**:
- `SuccessRate = (TotalRequests - FailedRequests) / TotalRequests`

**验证规则**:
- `FailCount` ≥ 3 → 标记 `Healthy = false`
- `SuccessRate` < 0.5 → 考虑降低权重
- `LastCheck` 超过2分钟 → 可能检查器故障

**状态转换**:
```
健康(Healthy=true) --[连续3次失败]--> 不健康(Healthy=false)
不健康 --[探测成功]--> 健康
       --[60秒内无成功]--> 移除
```

**关系**:
- 关联一个 `NodeConfig`（一对一）
- 被 `HealthChecker` 维护

---

### 4. ConnectionPool - 连接池

为单个trojan节点管理trojan客户端连接。

**字段**:
```go
type ConnectionPool struct {
    Node         *NodeConfig      // 关联的节点配置
    IdleConns    chan *TrojanConn // 空闲连接通道
    ActiveCount  int              // 当前活跃连接数
    MaxIdle      int              // 最大空闲连接数，默认5
    MaxActive    int              // 最大活跃连接数，默认10
    Timeout      time.Duration    // 连接超时，默认10秒
    mutex        sync.Mutex       // 并发保护
    closed       bool             // 是否已关闭
}
```

**操作**:
- `Get() (*TrojanConn, error)`: 获取连接（优先复用空闲连接）
- `Put(*TrojanConn)`: 归还连接到池
- `Close()`: 关闭池并清理所有连接

**验证规则**:
- `ActiveCount` ≤ `MaxActive`
- `len(IdleConns)` ≤ `MaxIdle`
- 获取连接时若池满，阻塞或返回错误

**状态转换**:
```
创建 → 运行中 → 关闭(closed=true)
```

**关系**:
- 关联一个 `NodeConfig`（一对一）
- 包含多个 `TrojanConn`（一对多）

---

### 5. TrojanConn - Trojan连接

封装单个到trojan服务器的连接。

**字段**:
```go
type TrojanConn struct {
    Conn         net.Conn      // 底层TCP连接
    Node         *NodeConfig   // 所属节点
    CreatedAt    time.Time     // 创建时间
    LastUsedAt   time.Time     // 最后使用时间
    RequestCount int           // 已处理请求数
    Closed       bool          // 是否已关闭
}
```

**操作**:
- `Dial(target string) (net.Conn, error)`: 通过trojan隧道连接目标
- `Close()`: 关闭连接

**验证规则**:
- 连接空闲时间超过5分钟 → 主动关闭
- `RequestCount` 超过1000 → 考虑回收重建

**状态转换**:
```
建立连接 → 空闲 → 使用中 → 空闲 → ... → 关闭
```

**关系**:
- 被 `ConnectionPool` 管理
- 关联一个 `NodeConfig`

---

### 6. ClientConnection - 客户端连接

代表一个连接到代理服务器的客户端会话。

**字段**:
```go
type ClientConnection struct {
    ID            string        // 唯一标识
    RemoteAddr    string        // 客户端IP地址
    ConnectedAt   time.Time     // 连接建立时间
    CurrentNode   *NodeConfig   // 当前使用的trojan节点
    BytesSent     int64         // 已发送字节数
    BytesReceived int64         // 已接收字节数
    Status        string        // 连接状态: "active", "idle", "closed"
}
```

**状态转换**:
```
建立连接 → active → idle → active → ... → closed
```

**验证规则**:
- 空闲时间超过30秒 → 状态变为 `idle`
- 总活跃时间超过1小时 → 考虑记录告警

**关系**:
- 使用一个 `NodeConfig`（多对一）

---

### 7. HealthCheckConfig - 健康检查配置

**字段**:
```go
type HealthCheckConfig struct {
    Interval         time.Duration // 检查间隔，默认30秒
    Timeout          time.Duration // 探测超时，默认5秒
    FailureThreshold int           // 连续失败阈值，默认3次
}
```

**验证规则**:
- `Interval` 必须 ≤ 30秒（符合FR-006）
- `Timeout` 必须 < `Interval`
- `FailureThreshold` 建议 2-5 次

---

### 8. LoggingConfig - 日志配置

**字段**:
```go
type LoggingConfig struct {
    Level  string // 日志级别: "debug", "info", "warn", "error"
    File   string // 日志文件路径，空字符串表示输出到stdout
    MaxSize int   // 单文件最大大小(MB)，默认100
}
```

**验证规则**:
- `Level` 必须是有效值之一
- `File` 若非空，必须有写权限
- `MaxSize` 建议 10-1000 MB

---

### 9. RoutingRule - 路由规则

代表单条路由规则配置。

**字段**:
```go
type RoutingRule struct {
    Type    string // 规则类型: "DOMAIN", "DOMAIN-SUFFIX", "IP-CIDR", "GEOIP", "FINAL"
    Pattern string // 匹配模式: 域名/IP段/国家代码，FINAL类型为空
    Action  string // 路由动作: "DIRECT", "PROXY", "REJECT"
    Order   int    // 规则顺序（配置文件中的位置）
    HitCount int64 // 命中次数（统计用）
}
```

**验证规则**:
- `Type` 必须是有效的规则类型之一
- `Pattern` 对于非FINAL规则必须非空
- `Pattern` 格式必须与 `Type` 匹配：
  - DOMAIN: 完整域名，如 "google.com"
  - DOMAIN-SUFFIX: 域名后缀，如 "google.com" (匹配 *.google.com)
  - IP-CIDR: CIDR格式，如 "192.168.0.0/16"
  - GEOIP: 国家代码，如 "CN", "US"
  - FINAL: 默认规则，Pattern为空
- `Action` 必须是 "DIRECT", "PROXY", "REJECT" 之一
- `Order` 唯一且递增

**规则匹配逻辑**:
1. 按 `Order` 从小到大依次匹配
2. 第一条匹配成功的规则决定路由动作
3. 如果所有规则都不匹配，使用 FINAL 规则

**示例**:
```go
// DOMAIN规则
RoutingRule{Type: "DOMAIN", Pattern: "baidu.com", Action: "DIRECT", Order: 1}

// DOMAIN-SUFFIX规则
RoutingRule{Type: "DOMAIN-SUFFIX", Pattern: "google.com", Action: "PROXY", Order: 2}

// IP-CIDR规则
RoutingRule{Type: "IP-CIDR", Pattern: "192.168.0.0/16", Action: "DIRECT", Order: 3}

// GEOIP规则
RoutingRule{Type: "GEOIP", Pattern: "CN", Action: "DIRECT", Order: 4}

// FINAL默认规则
RoutingRule{Type: "FINAL", Pattern: "", Action: "PROXY", Order: 999}
```

---

### 10. GeoIPDatabase - 地理位置数据库

**字段**:
```go
type GeoIPDatabase struct {
    FilePath    string    // 数据库文件路径
    LoadedAt    time.Time // 加载时间
    EntryCount  int       // 条目数量
    Version     string    // 数据库版本
}
```

**验证规则**:
- `FilePath` 必须存在且可读
- 支持MaxMind GeoLite2或类似格式

---

## 实体关系图

```
ProxyConfig (1)
  ├─ contains ─> NodeConfig (*多个)
  │                │
  │                ├─ has ─> ConnectionPool (1)
  │                │           │
  │                │           └─ manages ─> TrojanConn (*多个)
  │                │
  │                └─ has ─> NodeStatus (1)
  │
  ├─ contains ─> HealthCheckConfig (1)
  │              LoggingConfig (1)
  │
  └─ contains ─> RoutingRule (*多个)

ClientConnection (*多个)
  └─ uses ─> NodeConfig (1)

HealthChecker (单例)
  └─ monitors ─> NodeStatus (*多个)

Router (单例)
  ├─ applies ─> RoutingRule (*多个)
  └─ uses ─> GeoIPDatabase (1，可选)
```

---

## 并发访问模式

### 读多写少
- `NodeConfig`: 配置重载时写，请求处理时读 → 使用 `sync.RWMutex`
- `NodeStatus`: 健康检查写，节点选择读 → 使用 `sync.RWMutex`

### 频繁读写
- `ConnectionPool.ActiveCount`: 每次获取/归还连接都修改 → 使用 `sync.Mutex`
- `ClientConnection`: 每个连接独立，无共享状态

### 只读（启动后不变）
- `HealthCheckConfig`
- `LoggingConfig`

---

## 数据持久化

### 持久化数据
- **配置文件** (`config.yaml`): 所有 `*Config` 结构
  - 格式: YAML
  - 更新: 手动编辑 + 自动重载

### 临时数据（仅内存）
- `NodeStatus`: 运行时状态，重启后重置
- `ConnectionPool`: 连接池，重启后重建
- `TrojanConn`: 活跃连接，重启后断开
- `ClientConnection`: 客户端会话，重启后断开

### 日志数据
- **应用日志** (`proxy.log`): 操作日志、错误日志
  - 格式: 结构化文本（JSON可选）
  - 轮转: 按大小轮转（默认100MB）

---

## 数据流

### 请求处理流程
```
1. ClientConnection 发起请求
2. ProxyServer 接收并解析目标地址（域名或IP）
3. Router 根据 RoutingRules 匹配路由动作
   3.1 如果动作是 DIRECT：直接连接目标，不使用代理
   3.2 如果动作是 REJECT：拒绝连接，返回错误
   3.3 如果动作是 PROXY：继续下一步
4. NodeSelector 根据 NodeStatus 选择健康的 NodeConfig
5. ConnectionPool.Get() 获取 TrojanConn
6. 通过 TrojanConn 转发请求
7. 更新 NodeStatus 统计信息
8. 更新 RoutingRule.HitCount
9. ConnectionPool.Put() 归还连接
10. 返回响应给 ClientConnection
```

### 路由规则匹配流程
```
1. Router 接收目标地址（域名或IP）
2. 如果是域名：
   2.1 遍历 RoutingRule（按 Order 排序）
   2.2 检查 DOMAIN 精确匹配
   2.3 检查 DOMAIN-SUFFIX 后缀匹配
3. 如果是IP或需要IP规则：
   3.1 DNS解析域名得到IP（如需要）
   3.2 检查 IP-CIDR 规则匹配
   3.3 查询 GeoIPDatabase 获取国家代码
   3.4 检查 GEOIP 规则匹配
4. 如果所有规则都不匹配，使用 FINAL 规则
5. 返回匹配规则的 Action
```

### 健康检查流程
```
1. HealthChecker 定时触发（每30秒）
2. 遍历所有 NodeConfig
3. 创建测试 TrojanConn
4. 记录延迟和成功/失败
5. 更新对应的 NodeStatus
6. 若失败次数达阈值，标记 Healthy=false
```

### 配置重载流程
```
1. ConfigWatcher 检测文件变化
2. 解析新的 ProxyConfig
3. 对比新旧 NodeConfig 列表
4. 新增节点: 创建 ConnectionPool + NodeStatus
5. 删除节点: 关闭 ConnectionPool
6. 修改节点: 更新 NodeConfig，保持连接池
```

---

## 内存估算

假设配置20个节点，10个并发客户端：

- `NodeConfig`: 20 × ~200 bytes = 4KB
- `NodeStatus`: 20 × ~150 bytes = 3KB
- `ConnectionPool`: 20 × ~100 bytes = 2KB
- `TrojanConn`: 20 nodes × 5 idle conns × 500 bytes = 50KB
- `ClientConnection`: 10 × ~200 bytes = 2KB
- 配置和其他: ~10KB

**总计**: ~71KB (不含连接缓冲区)

每个活跃代理连接的缓冲区约 ~64KB，10个并发 = 640KB

**整体内存估算**: ~1MB + 网络缓冲区（符合<100MB约束）

---

## 数据验证清单

- [x] 所有实体字段类型明确
- [x] 验证规则清晰
- [x] 状态转换定义
- [x] 关系已建模
- [x] 并发访问模式已考虑
- [x] 持久化策略已定义
- [x] 内存使用在预算内
