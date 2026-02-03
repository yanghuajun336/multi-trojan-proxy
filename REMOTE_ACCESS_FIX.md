# 远程访问问题诊断和解决

## 🎉 好消息

**本机测试成功！** 这证明Trojan协议实现完全正常！

```
$ curl -x http://103.79.78.155:8080 https://www.google.com
<HTML>...302 Moved...google.com.hk...</HTML>
```

✅ Trojan隧道工作正常
✅ TLS连接成功
✅ 协议握手完成

## ❌ 问题

从国内机器访问：
```
curl: (56) Recv failure: Connection reset by peer
```

## 🔍 原因分析

### 问题1: 监听地址配置错误（已修复）

**错误配置**：
```yaml
proxy:
  listen: "127.0.0.1:8080"  # ❌ 只监听本地
```

**正确配置**：
```yaml
proxy:
  listen: "0.0.0.0:8080"    # ✅ 监听所有接口
```

### 问题2: 服务未重启

修改配置后必须重启服务才能生效。

## 🔧 解决步骤

### 步骤1: 停止旧服务

```bash
# 查找进程
ps aux | grep proxy-server

# 示例输出：
# yanghuajun  166583  ... ./proxy-server

# 停止进程（使用实际的PID）
kill 166583
```

### 步骤2: 确认配置正确

```bash
# 检查配置
cat config.yaml | grep listen

# 应该看到：
# listen: "0.0.0.0:8080"
```

### 步骤3: 启动服务

```bash
cd /home/yanghuajun/code/proxy

# 启动服务
./build/proxy-server -config config.yaml &

# 或使用nohup后台运行
nohup ./build/proxy-server -config config.yaml > proxy.log 2>&1 &
```

### 步骤4: 验证监听状态

```bash
# 检查监听地址
netstat -tlnp | grep 8080
# 或
ss -tlnp | grep 8080

# 应该看到：
# tcp  0  0.0.0.0:8080  0.0.0.0:*  LISTEN  <PID>/proxy-server
# 或
# tcp  0  :::8080       :::*       LISTEN  <PID>/proxy-server
```

**关键检查**：
- ✅ 应该是 `0.0.0.0:8080` 或 `:::8080`
- ❌ 不应该是 `127.0.0.1:8080`

### 步骤5: 检查防火墙

```bash
# 检查ufw防火墙
sudo ufw status

# 如果防火墙开启且8080端口未放行
sudo ufw allow 8080/tcp

# 或检查iptables
sudo iptables -L -n | grep 8080

# 如果需要放行
sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
```

### 步骤6: 本机测试

```bash
# 先在本机测试
curl -x http://127.0.0.1:8080 https://www.google.com

# 也测试外部地址
curl -x http://103.79.78.155:8080 https://www.google.com
```

两个都应该成功。

### 步骤7: 远程测试

从国内机器测试：

```bash
# 测试TCP连接是否通
telnet 103.79.78.155 8080
# 或
nc -zv 103.79.78.155 8080

# 应该看到：
# Connection to 103.79.78.155 8080 port [tcp/*] succeeded!
```

如果连接成功，再测试代理：

```bash
curl -x http://103.79.78.155:8080 https://www.google.com
```

## 📋 完整检查清单

### 服务器端（103.79.78.155）

- [ ] config.yaml 中 `listen: "0.0.0.0:8080"`
- [ ] 旧的proxy-server进程已停止
- [ ] 新的proxy-server已启动
- [ ] 监听地址是 `0.0.0.0:8080` 或 `:::8080`
- [ ] 防火墙允许8080端口
- [ ] 本机可以访问Google（证明Trojan工作）

### 客户端（国内机器）

- [ ] 能telnet到 103.79.78.155:8080
- [ ] curl代理测试成功

## 🚨 常见问题

### Q1: 重启后还是127.0.0.1

**原因**：配置文件未生效或使用了错误的配置文件

**解决**：
```bash
# 明确指定配置文件
./build/proxy-server -config /home/yanghuajun/code/proxy/config.yaml
```

### Q2: telnet无法连接

**原因**：防火墙阻止或服务未监听

**解决**：
```bash
# 检查服务是否运行
ps aux | grep proxy-server

# 检查监听
netstat -tlnp | grep 8080

# 检查防火墙
sudo ufw status
sudo iptables -L -n
```

### Q3: 连接后立即断开

**原因**：可能是云服务商的安全组规则

**解决**：
- 检查云服务商控制台的安全组设置
- 确保8080端口入站规则开放

### Q4: 有时能连有时不能

**原因**：可能是负载均衡或健康检查问题

**解决**：
```bash
# 查看日志
tail -f proxy.log

# 或如果输出到stdout
./build/proxy-server -config config.yaml
```

## 🎯 快速修复命令

```bash
# 一键修复（在服务器上执行）
cd /home/yanghuajun/code/proxy

# 1. 停止旧服务
kill $(ps aux | grep '[p]roxy-server' | awk '{print $2}')

# 2. 确认配置
sed -i 's/127.0.0.1:8080/0.0.0.0:8080/' config.yaml

# 3. 启动新服务
nohup ./build/proxy-server -config config.yaml > proxy.log 2>&1 &

# 4. 验证监听
sleep 2
netstat -tlnp | grep 8080

# 5. 测试
curl -x http://127.0.0.1:8080 https://www.google.com
```

## 📊 预期结果

### 服务器端日志

```
[INFO] === Multi-Trojan Proxy v1.0.0 ===
[INFO] Configuration loaded from config.yaml
[INFO] Starting HTTP proxy server on 0.0.0.0:8080  # ← 注意这里
[INFO] Proxy server started successfully
[DEBUG] CONNECT request target=www.google.com:443
[DEBUG] Selected node: my-trojan
[DEBUG] tunneling connection target=www.google.com:443
[DEBUG] CONNECT tunnel closed duration=XXXms
```

### 客户端输出

```bash
$ curl -x http://103.79.78.155:8080 https://www.google.com
<!doctype html><html>...
# 或
<HTML><HEAD>...302 Moved...google.com.hk...</HTML>
```

都表示成功！

## 🔒 安全建议

### 1. 限制访问IP（可选）

如果只允许特定IP访问：

```bash
# 使用iptables
sudo iptables -A INPUT -p tcp --dport 8080 -s <允许的IP> -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8080 -j DROP
```

### 2. 使用认证（未实现）

当前代理没有认证机制，任何人知道地址都能使用。

建议：
- 使用防火墙限制访问IP
- 或在Nginx等反向代理后添加认证
- 或考虑实现HTTP代理认证

### 3. 监控日志

```bash
# 定期检查日志
tail -100 proxy.log

# 查找异常
grep ERROR proxy.log
grep WARN proxy.log
```

## 📝 总结

1. **主要问题**：监听地址配置错误
2. **解决方案**：改为 `0.0.0.0:8080` 并重启
3. **验证方法**：telnet测试 + curl代理测试
4. **成功标志**：国内机器能访问Google

---

**修复后重新测试，应该就可以了！** 🚀
