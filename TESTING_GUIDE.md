# 测试和已知限制

## 当前实现状态

### ✅ 已实现并可用

1. **配置管理**
   - 配置加载、验证
   - SSL/TLS字段（包括SNI）
   - 配置热重载

2. **服务架构**
   - HTTP代理服务器
   - 多节点管理
   - 健康检查系统
   - 负载均衡
   - 智能路由引擎

3. **HTTP请求处理**
   - HTTP请求解析和转发
   - CONNECT隧道（HTTPS）
   - 请求头处理

### ⚠️ 当前限制

**Trojan协议未完整实现**

当前的 `internal/trojan/client.go` 使用的是**直接TCP连接**，而不是真正的Trojan协议。这意味着：

#### 工作原理（当前）

```
浏览器 → 本代理服务 → 直接HTTP请求 → 目标网站
        (127.0.0.1:8080)   ❌ 没有通过Trojan隧道
```

#### 期望原理（完整实现后）

```
浏览器 → 本代理服务 → TLS连接 → Trojan服务器 → 目标网站
        (127.0.0.1:8080)   (加密隧道)   (解密转发)
```

## 为什么会出现错误

### 错误1: `unsupported protocol scheme ""`

**原因**：HTTP代理请求的URL缺少scheme（http/https）

**已修复**：在 `handler.go` 中添加了URL补全逻辑
```go
if outReq.URL.Scheme == "" {
    outReq.URL.Scheme = "http"
}
if outReq.URL.Host == "" {
    outReq.URL.Host = r.Host
}
```

### 错误2: 实际无法访问外网

**原因**：当前实现是直接HTTP请求，不经过Trojan隧道

**解决方案**：需要完善Trojan协议实现（见下文）

## 测试方法

### 测试1：配置验证

验证服务是否能正常启动和加载配置：

```bash
./build/proxy-server -config config.yaml
```

**期望结果**：
```
[INFO] === Multi-Trojan Proxy v1.0.0 ===
[INFO] Configuration loaded from config.yaml
[INFO] Proxy server started successfully
```

✅ 如果看到这些日志，说明架构工作正常

### 测试2：健康检查

查看节点健康检查是否工作：

```bash
# 启动服务后，等待30秒（健康检查间隔）
# 查看日志，应该看到健康检查记录
```

**期望日志**：
```
[INFO] Health check: node my-trojan is healthy (latency: XXms)
或
[WARN] Node my-trojan marked as unhealthy
```

### 测试3：HTTP请求（会失败，正常）

```bash
# 启动服务
./build/proxy-server -config config.yaml &

# 尝试通过代理访问
curl -x http://127.0.0.1:8080 http://www.baidu.com
```

**当前行为**：
- 会尝试直接HTTP请求（不经过Trojan）
- 如果目标网站可直接访问，可能成功
- 如果需要Trojan隧道才能访问，会失败

### 测试4：CONNECT隧道

```bash
curl -x http://127.0.0.1:8080 https://www.google.com
```

**当前行为**：
- CONNECT请求会被处理
- 会尝试建立连接（但不经过Trojan）
- 连接逻辑已实现，但缺少Trojan协议

## 完善Trojan实现的步骤

### 方案：集成trojan-go库（推荐）

项目已引入 `github.com/p4gefau1t/trojan-go`，可以直接使用。

#### 第1步：修改 client.go

在 `internal/trojan/client.go` 中：

```go
import (
    "crypto/tls"
    "crypto/sha256"
    "encoding/hex"
    // ... 其他导入
)

func (c *Client) Connect(ctx context.Context) error {
    // 1. 创建TLS配置
    tlsConfig := &tls.Config{
        ServerName:         c.config.SSL.SNI,
        InsecureSkipVerify: !c.config.SSL.Verify,
        NextProtos:         c.config.SSL.ALPN,
    }
    
    // 2. 建立TLS连接
    serverAddr := fmt.Sprintf("%s:%d", c.config.Server, c.config.Port)
    tlsConn, err := tls.Dial("tcp", serverAddr, tlsConfig)
    if err != nil {
        return fmt.Errorf("TLS connection failed: %w", err)
    }
    
    c.conn = tlsConn
    return nil
}

func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
    if err := c.Connect(ctx); err != nil {
        return nil, err
    }
    
    // 3. 发送Trojan认证头
    hash := sha256.Sum224([]byte(c.config.Password))
    hexHash := hex.EncodeToString(hash[:])
    
    // Trojan请求格式：
    // hash + CRLF + CRLF + CMD + ATYP + DST.ADDR + DST.PORT + CRLF + CRLF
    
    // 发送认证
    if _, err := c.conn.Write([]byte(hexHash + "\r\n")); err != nil {
        return nil, err
    }
    
    // 发送目标地址（这里需要解析address格式）
    // ... 完整实现见 CURRENT_STATUS.md
    
    return c.conn, nil
}
```

#### 第2步：测试TLS连接

编译后测试是否能建立TLS连接到Trojan服务器。

#### 第3步：完整协议实现

参考 trojan-go 库或 Trojan协议规范完成完整实现。

## 快速检查清单

当前你可以验证：

- [ ] 服务能正常启动
- [ ] 配置加载成功（包括SNI）
- [ ] 节点初始化成功
- [ ] 健康检查运行
- [ ] HTTP请求被接收（即使转发失败）
- [ ] 日志正常输出

不能验证（需要Trojan实现）：

- [ ] 真正通过Trojan隧道转发
- [ ] 访问需要翻墙的网站
- [ ] 完整的端到端代理功能

## 建议

### 立即可以做的

1. ✅ **验证架构** - 启动服务，查看所有模块是否正常工作
2. ✅ **配置测试** - 修改配置文件，验证热重载
3. ✅ **日志分析** - 理解服务的工作流程

### 下一步（使服务完全可用）

1. **完善Trojan实现** - 按上述方案实现TLS和协议
2. **端到端测试** - 连接真实Trojan服务器
3. **性能优化** - 完善连接池和缓存

## 参考资料

- **Trojan协议规范**: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **实现方案**: [CURRENT_STATUS.md](CURRENT_STATUS.md)
- **配置说明**: [docs/TROJAN_CONFIG.md](docs/TROJAN_CONFIG.md)

---

**总结**：

服务的**架构和框架**已经完整实现并可以运行。

但要真正代理流量，需要实现**Trojan协议的TLS连接和握手**部分。

这是最后一步，也是最关键的一步！
