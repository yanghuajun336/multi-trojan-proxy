# 实现原理与架构设计

## 概述

本项目是一个**多Trojan节点统一代理服务器**，为多个Trojan客户端提供统一的HTTP/HTTPS代理入口，并支持负载均衡、故障转移和智能路由。

## 核心原理

### 工作流程

```
用户设备 → HTTP代理 → 路由决策 → [直连 OR Trojan隧道] → 目标网站
           (本服务)    (智能路由)   (负载均衡/健康检查)
```

### 详细流程

1. **客户端请求**
   - 用户设备配置HTTP代理指向本服务（如 `http://127.0.0.1:8080`）
   - 发送HTTP/HTTPS请求到代理服务器

2. **路由决策**
   - 根据目标域名/IP匹配路由规则
   - 决定动作：DIRECT（直连）、PROXY（代理）、REJECT（拒绝）

3. **节点选择**（PROXY动作）
   - 从健康节点中按权重选择Trojan节点
   - 排除不健康节点，实现故障转移

4. **Trojan连接**
   - 建立到Trojan服务器的TLS加密连接
   - 发送Trojan协议握手（密码认证 + 目标地址）
   - 通过加密隧道转发流量

5. **数据转发**
   - 双向复制数据流：客户端 ↔ Trojan隧道 ↔ 目标服务器

## 核心模块

### 1. 代理服务器 (internal/proxy)

**职责**：处理HTTP/HTTPS代理请求

#### server.go
- 创建HTTP服务器，监听指定端口
- 管理并发连接数限制

#### handler.go
- 处理HTTP请求（普通HTTP）
- 处理CONNECT请求（HTTPS隧道）
- 集成路由引擎和故障转移

#### forwarder.go
- 实现数据转发逻辑
- 双向复制TCP流

#### failover.go
- 请求失败时自动重试其他节点
- 最多重试3次，避免级联失败

#### connections.go
- 跟踪活跃连接数
- 记录连接统计信息

### 2. Trojan客户端 (internal/trojan)

**职责**：管理Trojan节点连接和负载均衡

#### client.go
- **关键实现**：Trojan协议客户端
- 建立TLS加密连接（支持SNI、证书验证等）
- 发送Trojan握手（SHA224(密码) + 目标地址）
- 提供透明的网络连接接口

#### pool.go
- 为每个节点维护连接池
- 复用连接，提升性能
- 支持动态添加/删除节点

#### selector.go
- 实现加权轮询算法（Weighted Round Robin）
- 排除不健康节点
- 返回最优节点用于新请求

### 3. 健康检查 (internal/health)

**职责**：监控节点健康状态

#### checker.go
- 定期启动主动探测（默认30秒）
- 汇总主动和被动检测结果
- 更新节点健康状态

#### probe.go
- 主动探测：建立测试连接，测量延迟
- TCP连接测试（或完整HTTP请求测试）

#### passive.go
- 被动检测：记录实际请求的成功/失败
- 连续失败达阈值（默认3次）则标记不健康
- 快速响应节点异常

#### status.go
- 存储节点健康状态
- 记录延迟、成功率、失败次数等指标

#### query.go
- 提供健康状态查询接口
- 供监控或管理工具使用

### 4. 路由引擎 (internal/router)

**职责**：智能路由决策

#### router.go
- 按顺序匹配路由规则
- 返回动作（DIRECT/PROXY/REJECT）

#### domain.go
- DOMAIN：精确匹配域名
- DOMAIN-SUFFIX：匹配域名后缀（如 `.google.com`）

#### ipcidr.go
- IP-CIDR：匹配IP地址范围（如 `192.168.0.0/16`）

#### geoip.go
- GEOIP：根据IP查询国家代码（如 `CN`）
- 使用MaxMind GeoLite2数据库

#### dnscache.go
- DNS解析结果缓存（LRU）
- 加速重复域名解析

### 5. 配置管理 (internal/config)

**职责**：配置加载、验证、热重载

#### types.go
- 定义配置结构（Proxy、Node、Routing、HealthCheck、Logging）

#### loader.go
- YAML文件解析
- 配置验证（必填项、范围检查、逻辑一致性）
- 支持外部规则文件

#### watcher.go
- 监听配置文件变化（fsnotify）
- 计算配置差异（新增/删除/修改节点）
- 触发热重载

## Trojan协议实现

### 协议格式

Trojan协议在TLS加密层之上运行：

```
[Trojan Request]
+---------------------+----------+----------+
| SHA224(password)    | CRLF     | CRLF     |
| (56 bytes hex)      | (2 bytes)| (2 bytes)|
+---------------------+----------+----------+
| CMD (1 byte)        | ATYP     | DST.ADDR |
| 0x01=CONNECT        | (1 byte) | (vary)   |
+---------------------+----------+----------+
| DST.PORT (2 bytes)  | CRLF     | CRLF     |
| (big endian)        | (2 bytes)| (2 bytes)|
+---------------------+----------+----------+
| Payload                                   |
| (stream data)                             |
+-------------------------------------------+
```

### 连接流程

1. **TLS握手**
   ```
   客户端 → [Client Hello + SNI] → Trojan服务器
   客户端 ← [Server Hello + 证书] ← Trojan服务器
   ```

2. **Trojan认证**
   ```
   客户端 → [SHA224(password) + CRLF + CRLF] → Trojan服务器
   ```

3. **目标地址**
   ```
   客户端 → [CMD + ATYP + DST.ADDR + DST.PORT + CRLF + CRLF] → Trojan服务器
   ```

4. **数据传输**
   ```
   客户端 ↔ [加密数据流] ↔ Trojan服务器 ↔ 目标网站
   ```

### 关键配置

#### 必需配置
- `server`: Trojan服务器地址
- `port`: 服务器端口（通常443）
- `password`: 认证密码
- `sni`: TLS握手的SNI字段（**必需**，如 `bilibili.com`）

#### 可选配置（推荐默认值）
- `verify`: 是否验证服务器证书（默认 `false`）
- `verify_hostname`: 是否验证主机名（默认 `false`）
- `alpn`: 应用层协议协商（默认 `["h2", "http/1.1"]`）
- `cipher`: TLS 1.2密码套件
- `cipher_tls13`: TLS 1.3密码套件

## 数据流向

### HTTP请求流向

```
浏览器 → HTTP请求 → 代理服务器(本服务)
                      ↓
                 路由判断：DIRECT?
                      ↓ NO
                 选择Trojan节点
                      ↓
        建立TLS连接 → Trojan服务器
                      ↓
              发送Trojan握手
                      ↓
              转发HTTP请求 → 目标网站
                      ↓
              接收响应 ← 目标网站
                      ↓
        浏览器 ← 返回响应 ← 代理服务器
```

### HTTPS请求流向（CONNECT隧道）

```
浏览器 → CONNECT www.google.com:443 → 代理服务器
                                         ↓
                                    路由判断：DIRECT?
                                         ↓ NO
                                    选择Trojan节点
                                         ↓
              建立TLS连接 → Trojan服务器
                                         ↓
                                   发送目标地址
                                         ↓
浏览器 ← 200 Connection Established ← 代理服务器
         ↓
         | [浏览器TLS握手] → 代理 → Trojan → 目标网站
         | [加密数据流]    → 代理 → Trojan → 目标网站
         | [加密响应]      ← 代理 ← Trojan ← 目标网站
```

## 负载均衡算法

### 加权轮询 (Weighted Round Robin)

```python
# 伪代码
nodes = [NodeA(weight=10), NodeB(weight=5), NodeC(weight=8)]
weights = [10, 5, 8]  # 总权重 = 23

current_weight = [0, 0, 0]

def select_node():
    # 每次选择前，增加各节点的当前权重
    for i in range(len(nodes)):
        current_weight[i] += weights[i]
    
    # 选择当前权重最大的节点
    max_index = argmax(current_weight)
    selected = nodes[max_index]
    
    # 减少被选中节点的权重
    current_weight[max_index] -= sum(weights)
    
    return selected

# 结果分布接近 10:5:8
```

### 健康检查集成

- 不健康节点自动从候选列表中排除
- 恢复健康后自动重新加入
- 平滑过渡，不中断现有连接

## 故障转移策略

### 重试逻辑

```
尝试节点A → 失败
    ↓
尝试节点B → 失败
    ↓
尝试节点C → 成功
```

### 快速失败

- 单次连接超时：10秒
- 最大重试次数：3次
- 避免长时间阻塞用户请求

### 被动检测

- 实际请求失败立即记录
- 连续失败3次标记不健康
- 下次健康检查间隔前就能发现问题

## 配置热重载机制

### 监听流程

```
文件监听器(fsnotify)
    ↓
检测到配置文件修改
    ↓
重新加载配置文件
    ↓
计算配置差异
    ↓
应用变更：
  - 新增节点 → 创建连接池
  - 删除节点 → 关闭连接池
  - 修改节点 → 重建连接池
    ↓
不中断现有连接
```

### 平滑更新

- 新连接使用新配置
- 旧连接继续使用旧配置
- 旧连接完成后自然关闭

## 性能优化

### 连接池

- 复用Trojan连接，避免重复TLS握手
- 每个节点独立连接池
- 空闲连接自动清理

### DNS缓存

- LRU缓存DNS解析结果
- 减少重复域名查询
- 加速路由规则匹配

### 并发处理

- 每个客户端连接独立goroutine
- 并发连接数限制，防止过载
- 无锁数据结构（通过channel通信）

## 安全考虑

### TLS加密

- 所有Trojan连接使用TLS 1.2/1.3
- 支持完整的密码套件配置
- SNI伪装，隐藏真实访问目标

### 配置安全

- 密码不记录到日志
- 建议配置文件权限：600
- 支持环境变量注入敏感信息（待实现）

## 监控与运维

### 日志

- 分级日志：debug/info/warn/error
- 支持文件输出和stdout
- 自动日志轮转

### 状态查询

- 节点健康状态API（待暴露HTTP接口）
- 连接统计信息
- 路由规则命中次数

### Systemd集成

- 服务定义文件：`configs/proxy.service`
- 支持开机自启
- 优雅关闭和重启

## 当前实现状态

### ✅ 已实现
- HTTP/HTTPS代理服务器
- 多节点管理和连接池
- 加权负载均衡
- 主动+被动健康检查
- 智能路由引擎（5种规则类型）
- 配置热重载
- 并发连接管理

### ⚠️ 需要完善
- **Trojan协议完整实现**（当前为框架代码）
  - TLS连接建立
  - SNI配置
  - Trojan握手协议
- 性能优化（规则缓存、索引）
- HTTP管理接口
- 更详细的监控指标

## 依赖库

- `github.com/p4gefau1t/trojan-go`: Trojan协议实现（待集成）
- `gopkg.in/yaml.v3`: YAML配置解析
- `github.com/fsnotify/fsnotify`: 文件系统监听
- `github.com/oschwald/geoip2-golang`: GeoIP查询

## 下一步计划

1. **完善Trojan客户端实现**（最高优先级）
   - 集成trojan-go库
   - 支持完整的SSL/TLS配置
   - 实现完整协议握手

2. **性能优化**
   - 规则匹配结果缓存
   - 连接池优化

3. **运维工具**
   - HTTP管理接口
   - Metrics导出（Prometheus格式）
