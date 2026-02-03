# 编译和运行指南

## 环境准备

### 1. 安装 Go

本项目需要 Go 1.21 或更高版本。

#### Ubuntu/Debian

```bash
# 下载Go 1.21
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz

# 解压到 /usr/local
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

# 添加到PATH（添加到 ~/.bashrc 或 ~/.profile）
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export GOPATH=$HOME/go' >> ~/.bashrc
source ~/.bashrc

# 验证安装
go version
```

#### macOS

```bash
# 使用Homebrew
brew install go

# 或下载安装包
# https://go.dev/dl/
```

#### 其他Linux发行版

从官方下载：https://go.dev/dl/

### 2. 配置Go模块代理（可选，加速下载）

```bash
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GO111MODULE=on
```

## 编译项目

### 下载依赖

```bash
cd /home/yanghuajun/code/proxy

# 下载Go模块依赖
go mod download

# 整理依赖
go mod tidy
```

### 编译

```bash
# 方法1: 使用Makefile
make build

# 方法2: 手动编译
mkdir -p build
go build -o build/proxy-server ./cmd/proxy

# 方法3: 编译并安装到 $GOPATH/bin
go install ./cmd/proxy
```

### 验证编译

```bash
# 检查可执行文件
ls -lh build/proxy-server

# 查看帮助
./build/proxy-server -h
```

输出示例：
```
Usage of ./build/proxy-server:
  -config string
        Path to config file (default "config.yaml")
  -version
        Show version information
```

## 配置服务

### 创建配置文件

```bash
# 复制配置模板
cp configs/config.real.example.yaml config.yaml

# 编辑配置
vim config.yaml
```

### 配置Trojan节点

**最简配置** `config.yaml`：

```yaml
proxy:
  listen: "0.0.0.0:8080"
  timeout: 30s
  max_concurrent: 20

nodes:
  - name: "my-trojan"
    server: "xg.xgacc.top"              # 改为你的Trojan服务器地址
    port: 10111                          # 改为你的端口
    password: "your-password-here"       # 改为你的密码
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
  file: ""  # 空字符串表示输出到stdout
  max_size: 100
```

### 验证配置

```bash
# 检查配置文件语法（需要安装yq）
# sudo snap install yq
# ./scripts/validate-config.sh

# 或直接启动服务（会自动验证）
./build/proxy-server -config config.yaml
```

## 运行服务

### 前台运行（用于测试）

```bash
# 使用默认配置文件 config.yaml
./build/proxy-server

# 或指定配置文件
./build/proxy-server -config /path/to/config.yaml

# 使用debug日志级别（配置文件中设置 logging.level: debug）
./build/proxy-server -config config.yaml
```

输出示例：
```
2026-02-03 15:00:00 [INFO] Loading configuration from config.yaml
2026-02-03 15:00:00 [INFO] Starting proxy server on 0.0.0.0:8080
2026-02-03 15:00:00 [INFO] Health checker started, interval: 30s
2026-02-03 15:00:00 [INFO] Configuration watcher started
```

### 后台运行（使用nohup）

```bash
nohup ./build/proxy-server -config config.yaml > proxy.log 2>&1 &

# 查看进程
ps aux | grep proxy-server

# 查看日志
tail -f proxy.log

# 停止服务
pkill -f proxy-server
```

### 使用systemd（推荐生产环境）

1. **复制服务文件**

```bash
sudo cp configs/proxy.service /etc/systemd/system/
```

2. **编辑服务文件**

```bash
sudo vim /etc/systemd/system/proxy.service
```

修改以下路径：
```ini
[Service]
ExecStart=/home/yanghuajun/code/proxy/build/proxy-server -config /home/yanghuajun/code/proxy/config.yaml
WorkingDirectory=/home/yanghuajun/code/proxy
User=yanghuajun
```

3. **启动服务**

```bash
# 重新加载systemd配置
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start proxy

# 查看状态
sudo systemctl status proxy

# 设置开机自启
sudo systemctl enable proxy

# 查看日志
sudo journalctl -u proxy -f
```

4. **管理服务**

```bash
# 停止服务
sudo systemctl stop proxy

# 重启服务
sudo systemctl restart proxy

# 重新加载配置（无需重启，利用热重载功能）
# 直接修改config.yaml，服务会自动检测并重载

# 禁用开机自启
sudo systemctl disable proxy
```

## 测试代理

### 配置客户端

#### 浏览器配置

**Firefox**:
1. 设置 → 网络设置 → 手动代理配置
2. HTTP代理：`localhost` 端口：`8080`
3. 勾选"也用于HTTPS"

**Chrome/Edge**:
```bash
# Linux/macOS
chromium --proxy-server="http://localhost:8080"

# Windows
chrome.exe --proxy-server="http://localhost:8080"
```

#### 系统代理（Ubuntu）

```bash
# 临时设置
export http_proxy=http://localhost:8080
export https_proxy=http://localhost:8080

# 测试
curl https://www.google.com
```

### 测试命令

```bash
# 测试HTTP代理
curl -x http://localhost:8080 http://www.google.com

# 测试HTTPS代理（CONNECT隧道）
curl -x http://localhost:8080 https://www.google.com

# 测试直连规则（如果配置了路由规则）
curl -x http://localhost:8080 https://www.baidu.com

# 查看响应头
curl -x http://localhost:8080 -I https://www.google.com

# 测试速度
time curl -x http://localhost:8080 https://www.google.com > /dev/null
```

### 验证负载均衡（多节点配置）

```bash
# 发送多个请求，观察日志中的节点选择
for i in {1..10}; do
    curl -x http://localhost:8080 https://www.google.com > /dev/null
    echo "Request $i completed"
done

# 查看日志，应该看到不同节点被选中
tail -f proxy.log | grep "Selected node"
```

## 常见问题

### 编译错误

#### 问题：`go: command not found`
**解决**：按上述步骤安装Go

#### 问题：`package xxx is not in GOROOT`
**解决**：
```bash
go mod tidy
go mod download
```

#### 问题：依赖下载慢
**解决**：配置代理
```bash
go env -w GOPROXY=https://goproxy.cn,direct
```

### 运行错误

#### 问题：`bind: address already in use`
**原因**：端口8080已被占用

**解决**：
```bash
# 查找占用进程
sudo lsof -i :8080
sudo netstat -tulpn | grep 8080

# 杀死进程或修改config.yaml中的listen端口
```

#### 问题：`node[xxx].ssl.sni is required`
**原因**：配置文件中未设置SNI

**解决**：在每个节点配置中添加：
```yaml
ssl:
  sni: "bilibili.com"
```

#### 问题：`connection refused` 或 `timeout`
**原因**：
1. Trojan服务器地址/端口错误
2. 密码错误
3. 网络不通

**解决**：
1. 检查配置中的server、port、password
2. 测试网络连通性：`telnet xg.xgacc.top 10111`
3. 查看详细日志：设置 `logging.level: debug`

### 配置问题

#### 问题：代理不工作
**检查清单**：
- [ ] 配置文件语法正确（YAML格式）
- [ ] server、port、password正确
- [ ] ssl.sni已配置
- [ ] 至少有一个enabled: true的节点
- [ ] 服务已启动且无错误日志

#### 问题：直连规则不生效
**原因**：需要配置路由规则

**解决**：参考 `configs/config.example.yaml` 添加routing配置

## 日志查看

### 日志级别

- `debug`: 详细调试信息
- `info`: 一般信息（推荐）
- `warn`: 警告信息
- `error`: 仅错误信息

### 关键日志

```
# 启动成功
[INFO] Starting proxy server on 0.0.0.0:8080

# 节点选择
[DEBUG] Selected node: us-node-1 (weight: 10)

# 请求处理
[INFO] Handling request: www.google.com:443

# 健康检查
[INFO] Health check: node us-node-1 is healthy (latency: 120ms)

# 配置重载
[INFO] Configuration file changed, reloading...
[INFO] Added new node: jp-node-1
```

### 错误日志

```
# 节点连接失败
[ERROR] Failed to connect to node us-node-1: connection timeout

# 配置错误
[ERROR] Failed to load config: node[my-node].ssl.sni is required

# 健康检查失败
[WARN] Node us-node-1 marked as unhealthy (failures: 3)
```

## 性能调优

### 并发连接数

```yaml
proxy:
  max_concurrent: 100  # 根据服务器性能调整
```

### 连接池大小

当前连接池大小硬编码为10，可在 `internal/trojan/pool.go` 中修改：
```go
const maxConnections = 20  // 增加连接池大小
```

### 健康检查间隔

```yaml
health_check:
  interval: 60s  # 减少检查频率，降低开销
```

## 下一步

1. **完善Trojan协议实现**（见 CURRENT_STATUS.md）
2. **添加路由规则**（见 configs/config.example.yaml）
3. **监控和日志分析**
4. **性能测试和优化**

## 相关文档

- [当前状态说明](CURRENT_STATUS.md)
- [实现原理](docs/ARCHITECTURE.md)
- [Trojan配置说明](docs/TROJAN_CONFIG.md)
- [配置分离方案](docs/CONFIG_SPLIT.md)

---

**遇到问题？** 查看日志或提交Issue
