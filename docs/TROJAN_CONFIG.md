# Trojan 节点配置说明

## 配置字段对照

本服务的配置字段与标准Trojan客户端配置的对应关系：

### 必需字段

| 本服务字段 | Trojan标准字段 | 说明 | 示例 |
|-----------|---------------|------|------|
| `server` | `remote_addr` | Trojan服务器地址 | `"xg.xgacc.top"` |
| `port` | `remote_port` | Trojan服务器端口 | `10111` |
| `password` | `password[0]` | 认证密码 | `"2a17c69f-..."` |
| `ssl.sni` | `ssl.sni` | **SNI伪装域名（必需）** | `"bilibili.com"` |

### SSL/TLS 配置字段

| 本服务字段 | Trojan标准字段 | 默认值 | 说明 |
|-----------|---------------|--------|------|
| `ssl.verify` | `ssl.verify` | `false` | 是否验证服务器证书 |
| `ssl.verify_hostname` | `ssl.verify_hostname` | `false` | 是否验证主机名 |
| `ssl.sni` | `ssl.sni` | **必需** | TLS SNI字段，用于伪装流量 |
| `ssl.alpn` | `ssl.alpn` | `["h2", "http/1.1"]` | 应用层协议协商 |
| `ssl.reuse_session` | `ssl.reuse_session` | `true` | TLS会话复用 |
| `ssl.session_ticket` | `ssl.session_ticket` | `false` | 会话票据 |
| `ssl.cipher` | `ssl.cipher` | 自动 | TLS 1.2密码套件 |
| `ssl.cipher_tls13` | `ssl.cipher_tls13` | 自动 | TLS 1.3密码套件 |
| `ssl.cert` | `ssl.cert` | `""` | 客户端证书路径（可选） |

### 本服务特有字段

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `name` | 节点名称（用于标识和日志） | 必需 |
| `weight` | 负载均衡权重（1-100） | `10` |
| `enabled` | 是否启用此节点 | `true` |

## 配置示例

### 最简配置（推荐）

```yaml
nodes:
  - name: "my-node"
    server: "xg.xgacc.top"
    port: 10111
    password: "2a17c69f-b6cb-4d47-958d-0add6ef2f03c"
    ssl:
      sni: "bilibili.com"  # 必须配置
```

其他SSL字段将使用以下默认值：
- `verify: false`
- `verify_hostname: false`
- `alpn: ["h2", "http/1.1"]`
- `reuse_session: true`
- `session_ticket: false`

### 完整配置（可选）

```yaml
nodes:
  - name: "my-node"
    server: "xg.xgacc.top"
    port: 10111
    password: "2a17c69f-b6cb-4d47-958d-0add6ef2f03c"
    weight: 10
    enabled: true
    ssl:
      sni: "bilibili.com"
      verify: false
      verify_hostname: false
      alpn: ["h2", "http/1.1"]
      reuse_session: true
      session_ticket: false
      cipher: "ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:..."
      cipher_tls13: "TLS_AES_128_GCM_SHA256:TLS_CHACHA20_POLY1305_SHA256:..."
```

## 字段详解

### SNI（Server Name Indication）

**⚠️ 这是最重要的配置字段！**

- **作用**：TLS握手时声明的服务器名称，用于流量伪装
- **必需**：是，Trojan服务器通常会验证SNI
- **推荐值**：
  - `bilibili.com` - 哔哩哔哩
  - `www.microsoft.com` - 微软
  - `www.cloudflare.com` - Cloudflare
  - 或任何知名大流量网站

- **错误配置**：如果SNI不匹配，Trojan服务器可能拒绝连接

### verify 和 verify_hostname

- **作用**：验证服务器TLS证书的有效性
- **推荐值**：`false`
- **原因**：
  - 大多数Trojan服务器使用自签名证书
  - SNI伪装域名与实际证书不匹配
  - 启用验证会导致连接失败

### ALPN（Application-Layer Protocol Negotiation）

- **作用**：协商应用层协议
- **默认值**：`["h2", "http/1.1"]`
- **说明**：支持HTTP/2和HTTP/1.1，与正常HTTPS流量一致

### 密码套件（cipher）

- **作用**：指定TLS加密算法
- **默认值**：由Go TLS库自动选择
- **建议**：除非有特殊需求，使用默认值即可

**标准Trojan密码套件**：
```
TLS 1.2: ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:...
TLS 1.3: TLS_AES_128_GCM_SHA256:TLS_CHACHA20_POLY1305_SHA256:...
```

### TLS会话复用

- `reuse_session: true`：复用TLS会话，减少握手开销
- `session_ticket: false`：不使用会话票据（更安全）

## 标准Trojan配置转换示例

### 原始Trojan配置

```json
{
    "run_type": "client",
    "local_addr": "127.0.0.1",
    "local_port": 1080,
    "remote_addr": "xg.xgacc.top",
    "remote_port": 10111,
    "password": ["2a17c69f-b6cb-4d47-958d-0add6ef2f03c"],
    "ssl": {
        "verify": false,
        "verify_hostname": false,
        "sni": "bilibili.com",
        "alpn": ["h2", "http/1.1"]
    }
}
```

### 转换后的本服务配置

```yaml
proxy:
  listen: "0.0.0.0:8080"  # 本服务监听端口（非1080）
  timeout: 30s
  max_concurrent: 20

nodes:
  - name: "xg-node"
    server: "xg.xgacc.top"         # remote_addr
    port: 10111                     # remote_port
    password: "2a17c69f-b6cb-4d47-958d-0add6ef2f03c"  # password[0]
    ssl:
      sni: "bilibili.com"          # ssl.sni
      verify: false                 # ssl.verify
      verify_hostname: false        # ssl.verify_hostname
      alpn: ["h2", "http/1.1"]     # ssl.alpn

# ... 其他配置（健康检查、日志等）
```

## 多节点配置

**优势**：本服务支持多个Trojan节点，实现负载均衡和故障转移

```yaml
nodes:
  - name: "node-1"
    server: "us1.example.com"
    port: 443
    password: "password1"
    weight: 10  # 权重高，分配更多流量
    ssl:
      sni: "bilibili.com"
      
  - name: "node-2"
    server: "hk1.example.com"
    port: 443
    password: "password2"
    weight: 5   # 权重低，分配较少流量
    ssl:
      sni: "microsoft.com"
      
  - name: "node-3"
    server: "jp1.example.com"
    port: 443
    password: "password3"
    enabled: false  # 临时禁用，不参与负载均衡
    ssl:
      sni: "cloudflare.com"
```

**流量分配**：按权重比例 `10:5` 分配给 node-1 和 node-2

## 配置验证

启动服务时会自动验证配置：

### 必需字段检查
- ✅ server不能为空
- ✅ port必须在1-65535范围
- ✅ password不能为空
- ✅ **ssl.sni不能为空**

### 逻辑检查
- ✅ 至少有一个启用的节点
- ✅ 节点名称不能重复
- ✅ 权重必须大于0

### 错误示例

```yaml
# ❌ 错误：缺少SNI
nodes:
  - name: "bad-node"
    server: "example.com"
    port: 443
    password: "pwd"
    # ssl.sni未配置！

# 启动时报错：
# Error: node[bad-node].ssl.sni is required
```

## 常见问题

### Q: 为什么必须配置SNI？

A: Trojan协议基于TLS，SNI用于流量伪装。服务器通常会验证SNI是否在白名单中。

### Q: SNI应该填什么？

A: 通常填写大流量网站域名（如bilibili.com、microsoft.com），具体取决于你的Trojan服务器配置。

### Q: 为什么不需要local_addr和local_port？

A: 本服务作为统一代理，只需配置自己的监听地址（`proxy.listen`），不需要为每个Trojan节点配置本地端口。

### Q: 密码套件需要自己配置吗？

A: 通常不需要，使用默认值即可。Go的TLS库会自动选择安全的密码套件。

### Q: 如何测试配置是否正确？

A: 
1. 启动服务：`./proxy-server -config config.yaml`
2. 查看日志，确认节点连接成功
3. 测试代理：`curl -x http://localhost:8080 https://www.google.com`

## 参考资料

- [Trojan协议规范](https://trojan-gfw.github.io/trojan/protocol)
- [trojan-go文档](https://p4gefau1t.github.io/trojan-go/)
- [TLS SNI说明](https://en.wikipedia.org/wiki/Server_Name_Indication)
