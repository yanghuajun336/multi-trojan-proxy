# 快速开始：多Trojan客户端统一代理服务

本文档帮助你快速部署和使用代理服务。

---

## 前提条件

- Linux服务器（Ubuntu 20.04+ 推荐）
- Go 1.21+ 已安装
- 至少一个可用的Trojan服务器节点信息（服务器地址、端口、密码）
- 网络能够访问Trojan服务器

---

## 安装步骤

### 1. 下载代码

```bash
git clone <repository_url> proxy
cd proxy
```

### 2. 安装依赖

```bash
go mod download
```

### 3. 编译程序

```bash
go build -o proxy-server ./cmd/proxy
```

编译完成后会生成可执行文件 `proxy-server`。

---

## 配置

### 1. 创建配置文件

复制配置示例：

```bash
cp configs/config.example.yaml config.yaml
```

### 2. 编辑配置

编辑 `config.yaml`，至少配置以下内容：

```yaml
proxy:
  listen: "0.0.0.0:8080"  # 代理服务监听地址

nodes:
  - name: "node-1"
    server: "your.trojan.server.com"  # 你的Trojan服务器地址
    port: 443
    password: "your_trojan_password"   # 你的Trojan密码
    weight: 10
    enabled: true
```

**重要**: 将 `server` 和 `password` 替换为你实际的Trojan服务器信息。

### 3. 配置多个节点（可选）

```yaml
nodes:
  - name: "us-node-1"
    server: "us1.trojan.example.com"
    port: 443
    password: "password1"
    weight: 10
    enabled: true
    
  - name: "hk-node-1"
    server: "hk1.trojan.example.com"
    port: 443
    password: "password2"
    weight: 5
    enabled: true
```

---

## 启动服务

### 前台运行（测试用）

```bash
./proxy-server -config config.yaml
```

看到以下输出表示启动成功：

```
[INFO] Loading config from config.yaml
[INFO] Loaded 2 nodes
[INFO] Starting health checker...
[INFO] Proxy server listening on 0.0.0.0:8080
[INFO] Server started successfully
```

### 后台运行（生产环境）

```bash
nohup ./proxy-server -config config.yaml > proxy.log 2>&1 &
```

查看日志：

```bash
tail -f proxy.log
```

### 使用systemd管理（推荐）

创建服务文件 `/etc/systemd/system/proxy.service`：

```ini
[Unit]
Description=Multi-Trojan Proxy Service
After=network.target

[Service]
Type=simple
User=proxy
WorkingDirectory=/opt/proxy
ExecStart=/opt/proxy/proxy-server -config /opt/proxy/config.yaml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl start proxy
sudo systemctl enable proxy  # 开机自启
sudo systemctl status proxy  # 查看状态
```

---

## 使用代理

### 配置浏览器

以Chrome为例：

1. 打开设置 → 高级 → 系统 → 打开代理设置
2. 手动配置代理：
   - HTTP代理：`your_server_ip:8080`
   - HTTPS代理：`your_server_ip:8080`
3. 保存设置

### 配置系统代理（Linux）

```bash
export http_proxy=http://your_server_ip:8080
export https_proxy=http://your_server_ip:8080
```

测试代理：

```bash
curl -I https://www.google.com
```

### 使用curl

```bash
curl -x http://your_server_ip:8080 https://www.google.com
```

---

## 验证服务

### 1. 检查服务状态

```bash
# 查看进程
ps aux | grep proxy-server

# 查看端口
netstat -tlnp | grep 8080

# 或使用 ss
ss -tlnp | grep 8080
```

### 2. 测试代理连接

```bash
# 测试HTTP连接
curl -v -x http://localhost:8080 http://www.google.com

# 测试HTTPS连接
curl -v -x http://localhost:8080 https://www.google.com
```

成功的输出示例：

```
* Trying 127.0.0.1:8080...
* Connected to localhost (127.0.0.1) port 8080 (#0)
> GET http://www.google.com/ HTTP/1.1
...
< HTTP/1.1 200 OK
...
```

### 3. 查看日志

```bash
# 实时查看日志
tail -f /var/log/proxy/proxy.log

# 或查看最近50行
tail -n 50 /var/log/proxy/proxy.log
```

正常运行的日志示例：

```
[INFO] 2026-02-01 15:00:00 node=us-node-1 status=healthy latency=120ms
[INFO] 2026-02-01 15:00:05 client=192.168.1.100 target=google.com node=us-node-1
[INFO] 2026-02-01 15:00:30 node=hk-node-1 status=healthy latency=80ms
```

---

## 常见问题

### 1. 启动失败："bind: address already in use"

端口被占用，修改配置文件中的监听端口：

```yaml
proxy:
  listen: "0.0.0.0:9090"  # 改为其他端口
```

### 2. 代理连接失败

检查：
- Trojan服务器配置是否正确（地址、端口、密码）
- 网络是否能访问Trojan服务器
- 防火墙是否允许8080端口

调试：

```bash
# 启用调试日志
# 编辑 config.yaml
logging:
  level: "debug"

# 重启服务
sudo systemctl restart proxy
```

### 3. 所有节点都不健康

查看健康检查日志，可能原因：
- Trojan服务器不可达
- 密码错误
- 网络问题

临时解决：手动测试Trojan连接

### 4. 配置修改后未生效

配置文件会在60秒内自动重载，或手动重启：

```bash
sudo systemctl restart proxy
```

---

## 性能优化

### 1. 增加并发数

编辑 `config.yaml`：

```yaml
proxy:
  max_concurrent: 50  # 根据实际需求调整
```

### 2. 调整健康检查间隔

```yaml
health_check:
  interval: 60s  # 减少检查频率降低开销
```

### 3. 优化节点权重

根据节点性能设置权重：

```yaml
nodes:
  - name: "fast-node"
    weight: 20  # 高性能节点，更高权重
  - name: "slow-node"
    weight: 5   # 低性能节点，较低权重
```

---

## 安全建议

1. **保护配置文件**：

```bash
chmod 600 config.yaml
```

2. **限制访问**：仅允许可信IP访问代理端口

```bash
# 使用iptables
sudo iptables -A INPUT -p tcp --dport 8080 -s 192.168.1.0/24 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8080 -j DROP
```

3. **定期轮换密码**：定期更换Trojan服务器密码

4. **监控日志**：定期检查异常访问

---

## 监控和维护

### 查看节点状态

```bash
# 查看日志中的健康检查记录
grep "status=healthy" /var/log/proxy/proxy.log | tail -20
```

### 监控资源使用

```bash
# CPU和内存使用
top -p $(pgrep proxy-server)

# 网络连接数
netstat -an | grep :8080 | wc -l
```

### 日志轮转

配置logrotate `/etc/logrotate.d/proxy`：

```
/var/log/proxy/*.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
    create 0644 proxy proxy
}
```

---

## 下一步

- 阅读 [data-model.md](./data-model.md) 了解系统架构
- 查看 [contracts/config-schema.md](./contracts/config-schema.md) 了解完整配置选项
- 参考 [research.md](./research.md) 了解技术细节

---

## 获取帮助

遇到问题时：
1. 查看日志文件定位错误
2. 启用调试日志获取更多信息
3. 检查网络连通性和Trojan服务器状态
4. 查阅项目文档和issue列表
