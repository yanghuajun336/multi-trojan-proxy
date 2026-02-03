# 多Trojan客户端统一代理服务

一个支持多节点负载均衡和智能路由的Trojan代理服务器。

## 📋 项目简介

本项目为多个Trojan节点提供统一的HTTP/HTTPS代理入口，实现：

- ✅ **统一代理入口**：一个端口服务所有Trojan节点
- ✅ **负载均衡**：加权轮询，按权重分配流量
- ✅ **故障转移**：自动检测节点健康状态，故障节点自动切换
- ✅ **智能路由**：支持域名/IP/GeoIP规则，国内直连，国外代理
- ✅ **配置热重载**：修改配置无需重启服务
- ✅ **并发支持**：支持多客户端同时使用

## ⚠️ 当前状态

**配置和架构**：✅ 完整实现  
**Trojan协议**：⚠️ 需要完善（当前为框架代码）

详见：[CURRENT_STATUS.md](CURRENT_STATUS.md)

## 🚀 快速开始

### 1. 安装依赖

```bash
# 安装Go 1.21+（如果未安装）
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 2. 编译项目

```bash
cd /home/yanghuajun/code/proxy

# 下载依赖
go mod download

# 编译
make build
# 或
go build -o build/proxy-server ./cmd/proxy
```

### 3. 配置服务

```bash
# 复制配置模板
cp configs/config.real.example.yaml config.yaml

# 编辑配置，填入你的Trojan信息
vim config.yaml
```

**最简配置示例**：

```yaml
proxy:
  listen: "0.0.0.0:8080"

nodes:
  - name: "my-trojan"
    server: "xg.xgacc.top"              # 你的Trojan服务器
    port: 10111
    password: "your-password-here"      # 你的密码
    ssl:
      sni: "bilibili.com"                # ⚠️ 必须配置
      verify: false
      verify_hostname: false

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "info"
  file: ""
  max_size: 100
```

### 4. 运行服务

```bash
# 前台运行
./build/proxy-server -config config.yaml

# 后台运行
nohup ./build/proxy-server -config config.yaml > proxy.log 2>&1 &
```

### 5. 测试代理

```bash
# 配置代理
export http_proxy=http://localhost:8080
export https_proxy=http://localhost:8080

# 测试访问
curl https://www.google.com
```

详细说明见：[BUILD_AND_RUN.md](BUILD_AND_RUN.md)

## 📚 文档

- **[编译和运行指南](BUILD_AND_RUN.md)** - 完整的安装、配置、运行说明
- **[当前实现状态](CURRENT_STATUS.md)** - 项目状态和后续开发计划
- **[实现原理](docs/ARCHITECTURE.md)** - 详细的架构设计和原理说明
- **[Trojan配置说明](docs/TROJAN_CONFIG.md)** - 配置字段对照和转换指南
- **[配置分离方案](docs/CONFIG_SPLIT.md)** - 如何分离配置文件

## 🏗️ 架构设计

### 工作原理

```
用户设备 → 本服务(HTTP代理:8080) → 路由决策 → Trojan节点选择 → TLS隧道 → 目标网站
                                  ↓              ↓
                              DIRECT?       加权负载均衡
                                            健康检查
```

### 核心模块

1. **代理服务器** (`internal/proxy`) - HTTP/HTTPS代理处理
2. **Trojan客户端** (`internal/trojan`) - 节点管理、连接池、负载均衡
3. **健康检查** (`internal/health`) - 主动+被动检测节点状态
4. **路由引擎** (`internal/router`) - 智能分流决策
5. **配置管理** (`internal/config`) - 加载、验证、热重载

## 🔧 主要功能

### 多节点管理

```yaml
nodes:
  - name: "us-node"
    server: "us1.example.com"
    weight: 10  # 权重高，分配更多流量
    
  - name: "hk-node"
    server: "hk1.example.com"
    weight: 5   # 权重低，分配较少流量
```

### 智能路由

```yaml
routing:
  rules:
    # 国内网站直连
    - type: DOMAIN-SUFFIX
      pattern: "baidu.com"
      action: DIRECT
      
    # 国外网站代理
    - type: DOMAIN-SUFFIX
      pattern: "google.com"
      action: PROXY
      
    # 国内IP直连
    - type: GEOIP
      pattern: "CN"
      action: DIRECT
      
    # 默认走代理
    - type: FINAL
      action: PROXY
```

### 健康检查

- **主动探测**：每30秒检测节点连通性和延迟
- **被动检测**：实际请求失败时立即记录
- **自动切换**：不健康节点自动排除，恢复后自动加入

### 配置热重载

修改 `config.yaml` 后，服务自动检测并重载：
- 新增节点 → 自动创建连接池
- 删除节点 → 优雅关闭连接
- 修改节点 → 重建连接池

## 📊 配置示例

### 标准Trojan配置转换

**原始Trojan客户端配置**：
```json
{
    "remote_addr": "xg.xgacc.top",
    "remote_port": 10111,
    "password": ["2a17c69f-b6cb-4d47-958d-0add6ef2f03c"],
    "ssl": {
        "sni": "bilibili.com",
        "verify": false,
        "verify_hostname": false
    }
}
```

**转换为本服务配置**：
```yaml
nodes:
  - name: "my-node"
    server: "xg.xgacc.top"
    port: 10111
    password: "2a17c69f-b6cb-4d47-958d-0add6ef2f03c"
    ssl:
      sni: "bilibili.com"
      verify: false
      verify_hostname: false
```

详见：[docs/TROJAN_CONFIG.md](docs/TROJAN_CONFIG.md)

## 🛠️ 开发计划

### ✅ 已完成

- [x] 项目架构和模块设计
- [x] 配置管理（加载、验证、热重载）
- [x] 多节点管理和连接池
- [x] 健康检查系统
- [x] 加权负载均衡
- [x] 智能路由引擎
- [x] 并发连接管理
- [x] 完整文档

### ⚠️ 待完善

- [ ] **Trojan协议完整实现**（最高优先级）
  - [ ] TLS连接建立
  - [ ] SNI配置应用
  - [ ] Trojan握手协议
- [ ] 单元测试和集成测试
- [ ] 性能优化
- [ ] HTTP管理接口

## 📝 配置字段说明

### 核心字段

| 字段 | 必需 | 默认值 | 说明 |
|------|------|--------|------|
| `server` | ✅ | - | Trojan服务器地址 |
| `port` | ✅ | - | 服务器端口 |
| `password` | ✅ | - | 认证密码 |
| `ssl.sni` | ✅ | - | **SNI伪装域名（必需）** |
| `ssl.verify` | ❌ | `false` | 是否验证证书 |
| `weight` | ❌ | `10` | 负载均衡权重 |
| `enabled` | ❌ | `true` | 是否启用 |

### 默认值

大部分字段有合理默认值，最简配置只需：
- `server`, `port`, `password`
- `ssl.sni`

其他字段可选，使用默认值即可。

## 🎯 使用场景

### 场景1：多节点负载均衡

你有3个Trojan服务器，希望自动分配流量：

```yaml
nodes:
  - {name: "us", server: "us.example.com", port: 443, password: "pwd1", weight: 10}
  - {name: "hk", server: "hk.example.com", port: 443, password: "pwd2", weight: 5}
  - {name: "jp", server: "jp.example.com", port: 443, password: "pwd3", weight: 8}
```

流量按 `10:5:8` 分配。

### 场景2：故障自动切换

节点us故障，流量自动切换到hk和jp，us恢复后自动加入。

### 场景3：智能分流

国内网站直连，节省流量；国外网站走代理，突破限制。

## 🐛 常见问题

### Q: 编译失败？
A: 检查Go版本 `go version`，需要1.21+

### Q: 启动失败？
A: 检查 `ssl.sni` 是否配置，这是必需字段

### Q: 代理不工作？
A: 
1. 检查配置正确性
2. 查看日志：`logging.level: debug`
3. 测试Trojan服务器连通性

### Q: 如何多节点？
A: 在 `nodes` 列表中添加多个节点，自动负载均衡

## 📄 许可证

见 [LICENSE](LICENSE) 文件

## 🤝 贡献

欢迎提交PR，特别是：
- 完善Trojan协议实现
- 添加测试用例
- 性能优化
- 文档改进

---

**最后更新**：2026-02-03  
**版本**：v0.1.0-alpha  
**状态**：开发中，架构完整，Trojan协议待实现
