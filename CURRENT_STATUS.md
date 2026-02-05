# 当前实现状态与使用说明

## ⚠️ 重要提示

### 当前实现状态

**配置管理和架构：✅ 完整实现**
- ✅ 配置加载、验证、热重载
- ✅ 多节点管理和连接池
- ✅ 健康检查（主动+被动）
- ✅ 加权负载均衡
- ✅ 智能路由引擎
- ✅ 并发连接管理

**Trojan协议实现：⚠️ 需要完善**

当前Trojan客户端实现是**框架代码**，缺少以下关键部分：

1. **TLS连接建立** - 需要实现完整的TLS握手
2. **SNI配置应用** - SNI字段已在配置中定义，但未在连接时使用
3. **Trojan协议握手** - 需要发送认证头和目标地址

## 快速理解：这个项目是什么？

### 架构原理

```
用户设备 → 本服务(HTTP代理) → Trojan节点选择 → TLS加密隧道 → 目标网站
        配置代理8080        负载均衡+健康检查   Trojan协议
```

### 工作流程

1. **用户请求**：浏览器配置代理 `http://localhost:8080`
2. **路由决策**：根据域名/IP判断是直连还是走代理
3. **节点选择**：从多个Trojan节点中选择健康节点（加权轮询）
4. **Trojan连接**：建立TLS加密连接，发送Trojan握手
5. **数据转发**：透明转发用户流量到目标网站

### 核心优势

相比单个Trojan客户端，本服务提供：

- **统一入口**：多个Trojan节点共享一个代理端口
- **负载均衡**：按权重分配流量到不同节点
- **故障转移**：节点失败自动切换（<5秒）
- **智能路由**：国内直连、国外代理、广告拒绝
- **配置热重载**：修改配置无需重启服务

## 配置说明

### 最简配置

```yaml
proxy:
  listen: "0.0.0.0:8080"

nodes:
  - name: "my-node"
    server: "xg.xgacc.top"          # 你的Trojan服务器
    port: 10111
    password: "your-password-here"
    ssl:
      sni: "bilibili.com"            # ⚠️ 必须配置SNI
      verify: false
      verify_hostname: false

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "info"
  file: ""  # stdout
  max_size: 100
```

### 配置字段说明

详见 [docs/TROJAN_CONFIG.md](docs/TROJAN_CONFIG.md)

**关键点**：
- ✅ `ssl.sni` 是**必须**配置的，用于TLS流量伪装
- ✅ 大部分字段有合理默认值，只需配置核心字段
- ✅ 支持多节点，自动负载均衡

## 下一步开发计划

### 优先级 P0：完善Trojan协议实现

**当前问题**：`internal/trojan/client.go` 只有框架代码

**需要实现**：

1. **TLS连接**（使用crypto/tls）
   ```go
   tlsConfig := &tls.Config{
       ServerName: node.SSL.SNI,
       InsecureSkipVerify: !node.SSL.Verify,
       // ... 其他SSL配置
   }
   conn = tls.Dial("tcp", serverAddr, tlsConfig)
   ```

2. **Trojan握手**
   ```go
   // 1. 发送认证头：SHA224(password) + CRLF + CRLF
   // 2. 发送目标地址：CMD + ATYP + DST.ADDR + DST.PORT + CRLF + CRLF
   // 3. 开始数据转发
   ```

3. **集成trojan-go库**
   - 项目已引入 `github.com/p4gefau1t/trojan-go`
   - 可以直接使用其协议实现

### 实现方案建议

#### 方案一：手动实现（学习价值高）

```go
// internal/trojan/client.go

func (c *Client) Connect(ctx context.Context) error {
    // 1. 创建TLS配置
    tlsConfig := &tls.Config{
        ServerName:         c.config.SSL.SNI,
        InsecureSkipVerify: !c.config.SSL.Verify,
        NextProtos:         c.config.SSL.ALPN,
        // ...
    }
    
    // 2. 建立TLS连接
    serverAddr := fmt.Sprintf("%s:%d", c.config.Server, c.config.Port)
    tlsConn, err := tls.Dial("tcp", serverAddr, tlsConfig)
    if err != nil {
        return err
    }
    
    // 3. 发送Trojan认证头
    hash := sha256.Sum224([]byte(c.config.Password))
    hexHash := hex.EncodeToString(hash[:])
    if _, err := tlsConn.Write([]byte(hexHash + "\r\n")); err != nil {
        return err
    }
    
    c.conn = tlsConn
    return nil
}

func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
    if err := c.Connect(ctx); err != nil {
        return nil, err
    }
    
    // 发送目标地址（Trojan协议格式）
    // CMD (0x01) + ATYP + DST.ADDR + DST.PORT + CRLF + CRLF
    // ...
    
    return c.conn, nil
}
```

#### 方案二：使用trojan-go库（快速可用）

```go
import (
    "github.com/p4gefau1t/trojan-go/tunnel/trojan"
)

func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
    // 使用trojan-go的客户端实现
    // 配置映射到trojan-go的Config结构
    // ...
}
```

### 优先级 P1：测试和验证

完成Trojan实现后：

1. **单元测试**：测试TLS连接和协议握手
2. **集成测试**：连接真实Trojan服务器
3. **端到端测试**：完整代理流程验证

### 优先级 P2：性能优化

- 路由规则缓存
- 连接池优化
- DNS缓存调优

## 编译和运行

### 前提条件

```bash
# 安装Go 1.21+
go version

# 克隆项目
cd /home/yanghuajun/code/proxy
```

### 编译

```bash
# 使用Makefile
make build

# 或手动编译
go build -o build/proxy-server ./cmd/proxy
```

### 配置

```bash
# 复制配置模板
cp configs/config.real.example.yaml config.yaml

# 编辑配置，填入你的Trojan信息
vim config.yaml
```

**重要**：确保配置了 `ssl.sni` 字段！

### 运行

```bash
# 启动服务
./build/proxy-server -config config.yaml

# 查看日志
tail -f /var/log/proxy/proxy.log
```

### 测试

```bash
# 测试代理是否工作
curl -x http://localhost:8080 https://www.google.com

# 测试直连规则（需配置路由规则）
curl -x http://localhost:8080 https://www.baidu.com
```

## 当前可以做什么

虽然Trojan协议实现需要完善，但以下功能已经可用：

1. ✅ **配置管理**：加载、验证、热重载
2. ✅ **多节点管理**：添加/删除/修改节点
3. ✅ **健康检查**：主动探测节点状态
4. ✅ **路由规则**：智能分流（需完善Trojan实现后验证）
5. ✅ **日志和监控**：完整的日志记录

## 文档资源

- [实现原理](docs/ARCHITECTURE.md) - 详细的架构和原理说明
- [Trojan配置](docs/TROJAN_CONFIG.md) - 配置字段对照和说明
- [配置分离](docs/CONFIG_SPLIT.md) - 配置文件分离方案
- [快速开始](docs/quickstart.md) - 使用指南
- [完整实施报告](IMPLEMENTATION_FINAL_REPORT.md) - 开发总结

## 如何贡献

欢迎贡献代码，特别是：

1. **完善Trojan协议实现**（优先级最高）
2. 添加单元测试和集成测试
3. 性能优化
4. 文档改进

## 许可证

见 [LICENSE](LICENSE) 文件

---

**最后更新**：2026-02-03
**状态**：架构完整，Trojan协议待实现
**建议**：先完善Trojan实现，再进行生产部署
