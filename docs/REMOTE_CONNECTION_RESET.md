# 远程访问Connection Reset问题诊断

## 问题现象

### 本地测试 ✅ 成功
```bash
$ curl -x http://127.0.0.1:8080 https://www.google.com
< HTTP/1.1 200 Connection Established
# 成功！
```

### 远程测试 ❌ 失败  
```bash
$ curl -x http://103.79.78.155:8080 https://www.google.com
* Recv failure: Connection reset by peer
```

## TCP抓包分析

从tcpdump抓包看到：

1. ✅ TCP三次握手完成
2. ✅ 收到CONNECT请求
3. ❌ 客户端发送RST重置连接

**关键**：服务端日志**没有任何记录**，说明请求没有到达应用层。

## 可能的原因

### 1. 防火墙/iptables规则问题

检查是否有规则导致连接被重置：

```bash
# 检查iptables规则
sudo iptables -L -n -v | grep 8080

# 检查连接跟踪
sudo conntrack -L | grep 8080
```

### 2. TCP层问题

可能的TCP配置问题：

```bash
# 检查TCP参数
sysctl net.ipv4.tcp_syncookies
sysctl net.ipv4.tcp_max_syn_backlog
```

### 3. 代理本身的问题

检查是否是hijack的问题：

```go
// 在handler.go的handleConnect中
hijacker, ok := w.(http.Hijacker)
if !ok {
    // 这里可能失败
}
```

## 详细诊断步骤

### 步骤1: 在服务器上抓包看完整流程

```bash
# 同时在服务器抓包
sudo tcpdump -i any port 8080 -w /tmp/proxy.pcap

# 从远程测试
# 从国内: curl -x http://103.79.78.155:8080 https://www.google.com

# 停止抓包并分析
sudo tcpdump -r /tmp/proxy.pcap -A
```

### 步骤2: 检查是否是内核参数问题

```bash
# 临时关闭TCP timestamps
sudo sysctl -w net.ipv4.tcp_timestamps=0

# 重新测试
```

### 步骤3: 使用strace跟踪

```bash
# 停止当前服务
kill <PID>

# 使用strace启动
strace -f -e trace=network -o /tmp/strace.log \
  ./build/proxy-server -config config.yaml &

# 从远程测试
# 查看strace日志
grep -A 10 "accept\|connect" /tmp/strace.log
```

### 步骤4: 添加更详细的日志

修改 `internal/proxy/handler.go`:

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 在最开始添加
    logger.Debug("=== REQUEST RECEIVED ===",
        "method", r.Method,
        "host", r.Host,
        "remote", r.RemoteAddr,
        "url", r.URL.String())
    
    // ... 原有代码
}
```

重新编译并测试。

## 解决方案尝试

### 方案1: 修改Hijacker实现

问题可能在于ResponseWriter的hijack。尝试添加更多错误处理：

```go
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
    logger.Debug("CONNECT request received",
        "target", r.Host,
        "remote", r.RemoteAddr)
    
    // 添加更多检查
    hijacker, ok := w.(http.Hijacker)
    if !ok {
        logger.Error("ResponseWriter does not support hijacking",
            "type", fmt.Sprintf("%T", w))
        http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
        return
    }
    
    // ... 继续
}
```

### 方案2: 检查是否是超时问题

在建立Trojan连接时可能超时：

```go
func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
    // 添加超时控制
    dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    conn, err := dialTrojan(c.config, address)
    // ...
}
```

### 方案3: 添加连接建立前的响应

尝试立即发送200响应，而不是等待Trojan连接：

```go
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
    // ... hijack ...
    
    // 立即发送200
    clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
    
    // 然后再建立Trojan连接
    targetConn, err := client.Dial(r.Context(), "tcp", r.Host)
    // ...
}
```

**当前代码是先建立Trojan连接，再发送200。如果Trojan连接慢，客户端可能超时。**

## 快速测试

### 测试1: 使用nc模拟

```bash
# 在服务器上
nc -l 9999

# 从国内
nc 103.79.78.155 9999
# 输入任意文字，看能否收到
```

### 测试2: 使用简单HTTP服务

```bash
# 在服务器上
python3 -m http.server 9999

# 从国内
curl http://103.79.78.155:9999
```

如果这些都正常，说明网络没问题，是代理逻辑的问题。

## 建议的修复

基于分析，最可能的问题是：

**Trojan连接建立太慢，客户端超时**

修改代码顺序：

```go
// 修改前：先连接再响应
targetConn, err := client.Dial(...)  // 慢
if err != nil {
    return
}
clientConn.Write([]byte("HTTP/1.1 200..."))  // 响应

// 修改后：先响应再连接
clientConn.Write([]byte("HTTP/1.1 200..."))  // 立即响应
targetConn, err := client.Dial(...)  // 再建立连接
if err != nil {
    clientConn.Close()
    return
}
```

这样客户端不会因为等待而超时。

## 实施步骤

1. 修改 `internal/proxy/handler.go` 的 `handleConnect` 方法
2. 调整连接建立的顺序
3. 重新编译测试

---

**关键发现**：本地工作但远程失败，通常是超时问题。
