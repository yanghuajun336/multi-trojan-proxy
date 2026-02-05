# ✅ Trojan协议实现完成 - 测试指南

## 🎉 完成情况

**已实现**：完整的Trojan协议支持！

### 新增文件
- `internal/trojan/protocol.go` - 完整的Trojan协议实现
  - TLS连接建立
  - SNI配置应用
  - SHA224密码认证
  - 目标地址编码
  - 完整协议握手

### 修改文件
- `internal/trojan/client.go` - 使用新的协议实现

## 📋 实现的功能

### Trojan协议支持

1. **TLS加密连接**
   - 使用配置的SNI（如 bilibili.com）
   - 支持证书验证控制
   - ALPN协议协商

2. **Trojan认证**
   - SHA224密码哈希
   - 十六进制编码
   - 完整的请求格式

3. **地址支持**
   - IPv4地址
   - IPv6地址  
   - 域名

4. **协议格式**
   ```
   [SHA224(password)] + CRLF +
   [CMD] + [ATYP] + [DST.ADDR] + [DST.PORT] + CRLF
   ```

## 🧪 测试步骤

### 1. 确认配置正确

编辑 `config.yaml`：

```yaml
proxy:
  listen: "0.0.0.0:8080"  # 或 127.0.0.1:8080

nodes:
  - name: "my-trojan"
    server: "xg.xgacc.top"              # 你的真实服务器地址
    port: 10111                          # 你的真实端口
    password: "2a17c69f-b6cb-4d47-958d-0add6ef2f03c"  # 你的真实密码
    weight: 10
    enabled: true
    ssl:
      sni: "bilibili.com"                # ⚠️ 必须与服务器配置一致
      verify: false
      verify_hostname: false
      alpn: ["h2", "http/1.1"]

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "debug"  # 使用debug级别查看详细信息
  file: ""
  max_size: 100
```

### 2. 启动服务

```bash
cd /home/yanghuajun/code/proxy

# 停止旧进程（如果有）
ps aux | grep proxy-server
# 找到PID并 kill <PID>

# 启动新服务
./build/proxy-server -config config.yaml
```

**期望日志**：
```
[INFO] === Multi-Trojan Proxy v1.0.0 ===
[INFO] Configuration loaded from config.yaml
[INFO] Loaded 1 nodes
[INFO] Node: my-trojan (xg.xgacc.top:10111) - weight=10
[INFO] Proxy server started successfully
[INFO] Listening on 0.0.0.0:8080
```

### 3. 测试HTTPS连接

在另一个终端：

```bash
# 测试Google（HTTPS）
curl -v -x http://103.79.78.155:8080 https://www.google.com

# 或使用环境变量
export http_proxy=http://103.79.78.155:8080
export https_proxy=http://103.79.78.155:8080
curl -v https://www.google.com
```

**成功的输出**：
```
* Connected to 103.79.78.155 (103.79.78.155) port 8080
* CONNECT www.google.com:443 HTTP/1.1
...
* Proxy replied 200 to CONNECT request
* SSL connection using TLSv1.3 / TLS_AES_256_GCM_SHA384
...
< HTTP/2 200
< content-type: text/html; charset=ISO-8859-1
...
<!doctype html><html>...
```

### 4. 测试HTTP连接

```bash
# 测试HTTP网站
curl -x http://103.79.78.155:8080 http://www.example.com
```

### 5. 查看代理日志

在代理服务器的输出中，你应该看到：

**成功的日志**：
```
[DEBUG] CONNECT request target=www.google.com:443
[DEBUG] Selected node: my-trojan (weight=10)
[DEBUG] tunneling connection target=www.google.com:443 node=my-trojan
[DEBUG] CONNECT tunnel closed target=www.google.com:443 duration=XXXms
```

**失败的日志**（如果有问题）：
```
[ERROR] failed to connect via trojan target=www.google.com:443 error=...
[ERROR] trojan dial failed: TLS dial failed: ...
[ERROR] trojan handshake failed: ...
```

## 🔍 故障排查

### 错误1: TLS dial failed

**可能原因**：
- Trojan服务器地址或端口错误
- 网络连接问题
- 防火墙阻止

**解决方案**：
```bash
# 测试网络连通性
telnet xg.xgacc.top 10111

# 或使用nc
nc -zv xg.xgacc.top 10111
```

### 错误2: 连接建立但无响应

**可能原因**：
- 密码错误
- SNI配置与服务器不匹配

**解决方案**：
1. 检查密码是否正确
2. 确认SNI配置（通常是 bilibili.com 或其他大网站）
3. 查看Trojan服务器日志

### 错误3: 仍然报TLS错误

**可能原因**：
- 配置文件未生效
- 服务未重启

**解决方案**：
```bash
# 确认使用新编译的版本
./build/proxy-server -version

# 完全重启
# 先停止，再启动
```

## 📊 性能测试

### 测试延迟

```bash
# 多次测试查看延迟
for i in {1..5}; do
  time curl -s -x http://103.79.78.155:8080 https://www.google.com > /dev/null
done
```

### 测试并发

```bash
# 10个并发请求
for i in {1..10}; do
  curl -s -x http://103.79.78.155:8080 https://www.google.com > /dev/null &
done
wait
```

## 🎯 验证清单

- [ ] 配置文件包含正确的server/port/password
- [ ] SNI配置正确
- [ ] 服务成功启动
- [ ] 能访问Google（HTTPS）
- [ ] 能访问其他网站
- [ ] 日志显示正常的隧道建立和关闭
- [ ] 没有TLS错误

## 🔧 高级配置

### 多节点测试

```yaml
nodes:
  - name: "node-1"
    server: "server1.example.com"
    port: 443
    password: "password1"
    weight: 10
    ssl:
      sni: "bilibili.com"
      
  - name: "node-2"
    server: "server2.example.com"
    port: 443
    password: "password2"
    weight: 5
    ssl:
      sni: "microsoft.com"
```

观察负载均衡：日志应显示交替使用不同节点。

### 添加路由规则

```yaml
routing:
  rules:
    # 国内直连
    - type: DOMAIN-SUFFIX
      pattern: "baidu.com"
      action: DIRECT
      
    # 国外代理
    - type: DOMAIN-SUFFIX
      pattern: "google.com"
      action: PROXY
      
    # 默认
    - type: FINAL
      action: PROXY
```

## 📝 下一步

### 如果测试成功

1. **浏览器配置** - 配置浏览器使用代理
2. **系统代理** - 设置系统级代理
3. **性能优化** - 根据使用情况调整配置
4. **部署到生产** - 使用systemd管理服务

### 如果测试失败

1. 查看详细错误日志
2. 检查Trojan服务器状态
3. 验证配置正确性
4. 参考 [TLS_ERROR_ANALYSIS.md](TLS_ERROR_ANALYSIS.md)

## 🎉 成功标志

当你看到以下输出时，说明完全成功：

```bash
$ curl -x http://103.79.78.155:8080 https://www.google.com
<!doctype html><html itemscope="" itemtype="http://schema.org/WebPage" lang="en">
<head><meta content="Search the world's information...
```

**恭喜！你的多Trojan代理服务器已经完全可用！** 🎊

---

## 📚 相关文档

- [TLS_ERROR_ANALYSIS.md](TLS_ERROR_ANALYSIS.md) - TLS错误分析
- [CURRENT_STATUS.md](CURRENT_STATUS.md) - 项目状态
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - 架构设计
- [BUILD_AND_RUN.md](BUILD_AND_RUN.md) - 编译运行指南
