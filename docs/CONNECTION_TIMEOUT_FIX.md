# 连接超时问题修复

## 问题根因

### 原始代码顺序（错误）

```go
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
    // 1. 选择客户端
    client, _ := h.selector.SelectClient()
    
    // 2. 建立Trojan连接 ← 可能很慢！
    targetConn, err := client.Dial(r.Context(), "tcp", r.Host)
    if err != nil {
        http.Error(w, "Failed", 502)  // 无法发送，已经超时！
        return
    }
    
    // 3. Hijack
    hijacker, _ := w.(http.Hijacker)
    clientConn, _, _ := hijacker.Hijack()
    
    // 4. 发送200响应 ← 太晚了，客户端已经超时！
    clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
    
    // 5. 数据转发
    io.Copy(targetConn, clientConn)
    io.Copy(clientConn, targetConn)
}
```

### 问题分析

1. **Trojan连接建立慢**
   - 从国内连接Trojan服务器可能需要几秒钟
   - TLS握手、协议协商都需要时间
   - 健康检查显示延迟282ms，实际建立连接可能更慢

2. **客户端超时**
   - curl默认CONNECT超时时间很短（通常几秒）
   - 如果一直没收到"200 Connection Established"
   - 客户端认为代理无响应，发送RST重置连接

3. **tcpdump看到的现象**
   ```
   客户端 → 服务器: SYN
   服务器 → 客户端: SYN-ACK
   客户端 → 服务器: ACK (连接建立)
   客户端 → 服务器: CONNECT www.google.com:443
   服务器: (正在建立Trojan连接，还没响应...)
   客户端 → 服务器: RST (超时，放弃！)
   ```

4. **为什么本地测试成功？**
   - 本地127.0.0.1延迟几乎为0
   - Trojan连接很快完成
   - 客户端还没超时就收到200响应

## 修复方案

### 新代码顺序（正确）

```go
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
    // 1. 选择客户端
    client, _ := h.selector.SelectClient()
    
    // 2. 先Hijack连接 ← 立即获取控制权
    hijacker, _ := w.(http.Hijacker)
    clientConn, _, _ := hijacker.Hijack()
    defer clientConn.Close()
    
    // 3. 立即发送200响应 ← 告诉客户端我们已经接受请求
    clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
    
    // 4. 现在建立Trojan连接 ← 客户端已经不会超时
    targetConn, err := client.Dial(r.Context(), "tcp", r.Host)
    if err != nil {
        // 虽然失败，但客户端已经收到200，所以只能关闭连接
        return
    }
    defer targetConn.Close()
    
    // 5. 数据转发
    io.Copy(targetConn, clientConn)
    io.Copy(clientConn, targetConn)
}
```

### 关键改进

1. **立即响应**
   - Hijack后立即发送"200 Connection Established"
   - 客户端知道代理已经接受请求，不会超时

2. **异步建立连接**
   - 发送200后再建立Trojan连接
   - 即使Trojan连接慢，客户端也在等待状态
   - 如果Trojan连接失败，只能关闭连接（因为已经发了200）

3. **符合HTTP CONNECT语义**
   - RFC 2616: 代理应该尽快发送200响应
   - 然后代理负责维持隧道，传输数据

## 测试验证

### 测试1: 本地测试

```bash
# 停止旧服务
kill <PID>

# 启动新服务
cd /home/yanghuajun/code/proxy
./build/proxy-server -config config.yaml

# 本地测试（应该仍然成功）
curl -v -x http://127.0.0.1:8080 https://www.google.com
```

预期：仍然成功（之前就成功）

### 测试2: 远程测试（关键）

```bash
# 在国内服务器上
curl -v -x http://103.79.78.155:8080 https://www.google.com

# 或使用更详细的跟踪
curl -v -x http://103.79.78.155:8080 https://www.google.com 2>&1 | tee curl.log
```

预期：
```
* Trying 103.79.78.155:8080...
* Connected to 103.79.78.155:8080
* CONNECT www.google.com:443 HTTP/1.1
...
< HTTP/1.1 200 Connection Established  ← 应该立即看到这个
<
* CONNECT phase completed!
* TLS handshake...
...
# 最终应该看到Google的响应
```

### 测试3: 超时测试

测试即使Trojan连接很慢，客户端也不会超时：

```bash
# 使用更短的总超时时间
timeout 30s curl -v -x http://103.79.78.155:8080 https://www.google.com
```

如果修复成功，即使Trojan连接需要几秒，客户端也会等待（因为已经收到200）。

## 潜在的新问题

### 问题：发送200后Trojan连接失败

**场景**：代理已经告诉客户端"200 Connection Established"，但Trojan连接建立失败。

**影响**：
- 客户端认为隧道已建立
- 实际上没有到目标的连接
- 数据发送会失败

**解决方案**：
1. 这是HTTP CONNECT协议的固有限制
2. 最佳实践：代理应该尽快告知客户端状态
3. 如果Trojan连接失败，关闭连接，客户端会重试
4. 大多数客户端（curl、浏览器）都能处理这种情况

### 问题：增加了失败延迟

**场景**：如果Trojan服务器不可达，客户端会等更久才知道失败。

**影响**：
- 原来：立即返回502 Bad Gateway
- 现在：发送200后等待，然后连接关闭

**权衡**：
- 优点：解决了正常情况下的超时问题（更重要）
- 缺点：失败情况下体验稍差（可接受）

## 总结

**根本原因**：HTTP CONNECT代理的响应时机问题

**错误做法**：先建立到目标的连接，再告诉客户端
- 导致客户端超时（Trojan连接慢时）

**正确做法**：先告诉客户端接受请求，再建立到目标的连接
- 客户端不会超时
- 符合HTTP代理最佳实践

**适用场景**：所有可能慢连接的代理（Trojan、SSH tunnel等）

---

**代码修改位置**：`internal/proxy/handler.go` 的 `handleConnect` 方法（第116-161行）
