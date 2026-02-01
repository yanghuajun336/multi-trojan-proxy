# 配置文件Schema定义

**版本**: 1.0  
**格式**: YAML  
**文件名**: config.yaml  
**用途**: 定义代理服务的所有配置参数

---

## 完整配置示例

```yaml
# 代理服务配置
proxy:
  # 监听地址和端口
  listen: "0.0.0.0:8080"
  
  # HTTP请求超时时间
  timeout: 30s
  
  # 最大并发连接数
  max_concurrent: 20

# Trojan节点配置列表
nodes:
  - name: "us-node-1"
    server: "us1.trojan.example.com"
    port: 443
    password: "your_password_here"
    weight: 10
    enabled: true
    
  - name: "hk-node-1"
    server: "hk1.trojan.example.com"
    port: 443
    password: "your_password_here"
    weight: 5
    enabled: true

# 路由规则配置
routing:
  # GeoIP数据库文件路径（可选）
  geoip_database: "/usr/share/GeoIP/GeoLite2-Country.mmdb"
  
  # 路由规则列表（按顺序匹配）
  rules:
    # 域名完全匹配规则
    - type: DOMAIN
      pattern: "baidu.com"
      action: DIRECT
      
    # 域名后缀匹配规则
    - type: DOMAIN-SUFFIX
      pattern: "google.com"
      action: PROXY
      
    - type: DOMAIN-SUFFIX
      pattern: "taobao.com"
      action: DIRECT
      
    # IP-CIDR规则（局域网直连）
    - type: IP-CIDR
      pattern: "192.168.0.0/16"
      action: DIRECT
      
    - type: IP-CIDR
      pattern: "10.0.0.0/8"
      action: DIRECT
      
    - type: IP-CIDR
      pattern: "172.16.0.0/12"
      action: DIRECT
      
    - type: IP-CIDR
      pattern: "127.0.0.0/8"
      action: DIRECT
      
    # GEOIP规则（中国IP直连）
    - type: GEOIP
      pattern: "CN"
      action: DIRECT
      
    # 默认规则（必须是最后一条）
    - type: FINAL
      action: PROXY

# 健康检查配置
health_check:
  # 检查间隔（不超过30秒）
  interval: 30s
  
  # 单次探测超时
  timeout: 5s
  
  # 连续失败阈值
  failure_threshold: 3

# 日志配置
logging:
  # 日志级别: debug, info, warn, error
  level: "info"
  
  # 日志文件路径
  file: "/var/log/proxy/proxy.log"
  
  # 单个日志文件最大大小（MB）
  max_size: 100
```

---

## Schema定义

### 根对象

| 字段 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `proxy` | ProxyConfig | 是 | 代理服务配置 |
| `nodes` | NodeConfig[] | 是 | Trojan节点列表 |
| `routing` | RoutingConfig | 否 | 路由规则配置（无此项则所有流量走代理） |
| `health_check` | HealthCheckConfig | 否 | 健康检查配置（有默认值） |
| `logging` | LoggingConfig | 否 | 日志配置（有默认值） |

---

### RoutingConfig对象

路由规则配置。

| 字段 | 类型 | 必需 | 默认值 | 验证规则 |
|------|------|------|--------|----------|
| `geoip_database` | string | 否 | `""` | GeoIP数据库文件路径，为空则不支持GEOIP规则 |
| `rules` | RoutingRule[] | 是 | - | 至少包含一条FINAL规则 |

**示例**:
```yaml
routing:
  geoip_database: "/usr/share/GeoIP/GeoLite2-Country.mmdb"
  rules:
    - type: DOMAIN
      pattern: "baidu.com"
      action: DIRECT
    - type: FINAL
      action: PROXY
```

---

### RoutingRule对象

单条路由规则。

| 字段 | 类型 | 必需 | 验证规则 |
|------|------|------|----------|
| `type` | string | 是 | 必须是: `DOMAIN`, `DOMAIN-SUFFIX`, `IP-CIDR`, `GEOIP`, `FINAL` |
| `pattern` | string | 视类型 | FINAL类型不需要，其他类型必需 |
| `action` | string | 是 | 必须是: `DIRECT`, `PROXY`, `REJECT` |

**规则类型说明**:

1. **DOMAIN**: 完全匹配域名
   - `pattern`: 完整域名，如 "baidu.com"
   - 匹配: 仅匹配 "baidu.com"，不匹配 "www.baidu.com"

2. **DOMAIN-SUFFIX**: 域名后缀匹配
   - `pattern`: 域名后缀，如 "google.com"
   - 匹配: "google.com", "www.google.com", "mail.google.com" 等

3. **IP-CIDR**: IP地址段匹配
   - `pattern`: CIDR格式，如 "192.168.0.0/16"
   - 匹配: 所有在此IP段内的地址

4. **GEOIP**: 地理位置匹配
   - `pattern`: ISO 3166-1 alpha-2 国家代码，如 "CN", "US", "JP"
   - 需要 `geoip_database` 配置才能使用
   - 匹配: 所有属于该国家的IP地址

5. **FINAL**: 默认规则
   - 无需 `pattern`
   - 必须是规则列表的最后一条
   - 匹配: 所有未被其他规则匹配的请求

**动作说明**:

- **DIRECT**: 直接连接目标，不经过代理
- **PROXY**: 通过trojan节点代理连接
- **REJECT**: 拒绝连接，返回错误

**规则匹配顺序**:
- 规则按配置文件中的顺序依次匹配
- 第一条匹配成功的规则生效
- FINAL规则必须是最后一条，作为兜底

**示例规则**:
```yaml
rules:
  # 国内网站直连
  - type: DOMAIN-SUFFIX
    pattern: "baidu.com"
    action: DIRECT
    
  - type: DOMAIN-SUFFIX
    pattern: "taobao.com"
    action: DIRECT
    
  # 国外网站代理
  - type: DOMAIN-SUFFIX
    pattern: "google.com"
    action: PROXY
    
  - type: DOMAIN-SUFFIX
    pattern: "youtube.com"
    action: PROXY
    
  # 局域网直连
  - type: IP-CIDR
    pattern: "192.168.0.0/16"
    action: DIRECT
    
  - type: IP-CIDR
    pattern: "10.0.0.0/8"
    action: DIRECT
    
  # 中国IP直连
  - type: GEOIP
    pattern: "CN"
    action: DIRECT
    
  # 广告域名拒绝（可选）
  - type: DOMAIN-SUFFIX
    pattern: "ad.doubleclick.net"
    action: REJECT
    
  # 默认规则：其他所有流量走代理
  - type: FINAL
    action: PROXY
```

---

## 配置文件验证

### 启动时验证

系统启动时会验证配置文件，以下情况会导致启动失败：

1. **语法错误**: YAML格式错误
2. **必填字段缺失**: `proxy.listen`, `nodes`等
3. **字段值非法**: 端口超出范围，timeout为负数等
4. **节点名重复**: 多个节点使用相同的`name`
5. **无可用节点**: `nodes`列表为空或所有节点都`enabled=false`
6. **路由规则错误**:
   - 缺少FINAL规则
   - FINAL规则不是最后一条
   - GEOIP规则但未配置geoip_database
   - pattern格式错误（如IP-CIDR不符合CIDR格式）
   - 规则类型拼写错误

### 运行时验证

配置文件重载时的验证：

1. **新配置验证通过**: 应用新配置
2. **新配置验证失败**: 保持旧配置，记录错误日志

---

## 路由规则最佳实践

### 1. 规则排序建议

```yaml
rules:
  # 1. 特定域名规则（最高优先级）
  - type: DOMAIN
    pattern: "example.com"
    action: DIRECT
    
  # 2. 域名后缀规则
  - type: DOMAIN-SUFFIX
    pattern: "baidu.com"
    action: DIRECT
    
  # 3. IP规则（局域网）
  - type: IP-CIDR
    pattern: "192.168.0.0/16"
    action: DIRECT
    
  # 4. GEOIP规则
  - type: GEOIP
    pattern: "CN"
    action: DIRECT
    
  # 5. FINAL规则（最后）
  - type: FINAL
    action: PROXY
```

### 2. 常见配置模式

**模式1：国内直连，国外代理**
```yaml
rules:
  # 常见国内网站
  - type: DOMAIN-SUFFIX
    pattern: "baidu.com"
    action: DIRECT
  - type: DOMAIN-SUFFIX
    pattern: "taobao.com"
    action: DIRECT
  - type: DOMAIN-SUFFIX
    pattern: "qq.com"
    action: DIRECT
    
  # 局域网直连
  - type: IP-CIDR
    pattern: "192.168.0.0/16"
    action: DIRECT
  - type: IP-CIDR
    pattern: "10.0.0.0/8"
    action: DIRECT
    
  # 中国IP直连
  - type: GEOIP
    pattern: "CN"
    action: DIRECT
    
  # 其他走代理
  - type: FINAL
    action: PROXY
```

**模式2：白名单模式（仅代理指定网站）**
```yaml
rules:
  # 需要代理的网站
  - type: DOMAIN-SUFFIX
    pattern: "google.com"
    action: PROXY
  - type: DOMAIN-SUFFIX
    pattern: "youtube.com"
    action: PROXY
  - type: DOMAIN-SUFFIX
    pattern: "twitter.com"
    action: PROXY
    
  # 其他所有直连
  - type: FINAL
    action: DIRECT
```

### 3. GEOIP数据库

**下载GeoLite2数据库**:
```bash
# 从MaxMind下载（需要注册账号）
wget https://download.maxmind.com/app/geoip_download?...

# 或使用开源替代
wget https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb
```

**配置路径**:
```yaml
routing:
  geoip_database: "/usr/share/GeoIP/GeoLite2-Country.mmdb"
```

如果不配置或文件不存在，GEOIP规则将被忽略，系统会记录警告日志。

---

## 版本兼容性

**当前版本**: 1.0

未来版本可能新增规则类型和字段，但保证向后兼容：
- 新规则类型有明确语义
- 旧配置文件继续有效
- 不会删除现有规则类型

如有破坏性变更，将升级主版本号（如 2.0）并提供迁移指南。
