# TLS 错误分析和解决方案

## 错误信息

```
curl: (35) error:0A000438:SSL routines::tlsv1 alert internal error
```

## 这个错误的意义

🎉 **好消息**：这个错误证明代理架构正在工作！

❌ **问题**：Trojan协议未实现，导致TLS握手失败

## 详细流程分析

### 实际发生了什么

```
1. curl → CONNECT www.google.com:443 → 你的代理(103.79.78.155:8080)
   ✅ 代理收到请求

2. 代理 → 调用 client.Dial() → 尝试建立隧道
   ❌ 问题在这里！

3. 代理 → 200 Connection Established → curl
   ✅ curl认为隧道已建立

4. curl → 开始TLS握手 → "隧道"
   ❌ 失败！因为"隧道"不是真正的隧道
```

### 问题根源

查看 `internal/trojan/client.go` 的 `Dial()` 方法：

```go
func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
    // ... 
    
    // TODO: Implement full Trojan protocol handshake
    // 当前这里只是返回了一个普通的TCP连接！
    return c.conn, nil  // ❌ 这不是Trojan隧道
}
```

**当前实现**：
- `c.conn` 是一个普通的TCP连接（甚至可能是nil或错误的连接）
- 没有连接到Trojan服务器
- 没有TLS加密
- 没有Trojan握手

### 期望实现

```go
func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
    // 1. 建立到Trojan服务器的TLS连接
    tlsConfig := &tls.Config{
        ServerName: c.config.SSL.SNI,  // "bilibili.com"
        InsecureSkipVerify: !c.config.SSL.Verify,
    }
    
    serverAddr := fmt.Sprintf("%s:%d", c.config.Server, c.config.Port)
    tlsConn, err := tls.Dial("tcp", serverAddr, tlsConfig)
    if err != nil {
        return nil, err
    }
    
    // 2. 发送Trojan认证
    hash := sha256.Sum224([]byte(c.config.Password))
    hexHash := hex.EncodeToString(hash[:])
    tlsConn.Write([]byte(hexHash + "\r\n"))
    
    // 3. 发送目标地址 (address = "www.google.com:443")
    // 按Trojan协议格式发送
    // CMD(1字节) + ATYP(1字节) + DST.ADDR + DST.PORT + CRLF
    
    // 4. 返回这个TLS连接，作为隧道
    return tlsConn, nil
}
```

## 为什么会出现TLS错误

```
curl期望的流程：
curl → TLS握手 → www.google.com

实际发生的流程：
curl → TLS握手 → 无效的连接 → ❌ 失败
```

curl尝试和Google进行TLS握手，但实际上连接并没有正确建立到Google，所以握手失败。

## 测试验证

### 当前你可以看到的

1. **代理启动成功** ✅
   ```
   [INFO] Proxy server started successfully
   ```

2. **接收到请求** ✅
   ```
   [DEBUG] CONNECT request target=www.google.com:443
   ```

3. **尝试建立隧道** ✅
   ```
   [DEBUG] tunneling connection target=www.google.com:443
   ```

4. **TLS握手失败** ❌
   ```
   curl: (35) error:0A000438:SSL routines::tlsv1 alert internal error
   ```

### 查看代理日志

重新运行代理并查看详细日志：

```bash
# 停止之前的服务
ps aux | grep proxy-server
# 找到PID并停止

# 启动服务（debug模式）
./build/proxy-server -config config.yaml

# 在另一个终端测试
curl -v -x http://103.79.78.155:8080 https://www.google.com
```

**期望看到的日志**：
```
[DEBUG] CONNECT request target=www.google.com:443
[DEBUG] Selected node: my-trojan
[DEBUG] tunneling connection target=www.google.com:443
[ERROR] 或 [DEBUG] tunnel error (可能)
```

## 完整的Trojan实现代码

创建一个新文件来实现完整的Trojan协议：

### 步骤1：创建实现文件

在 `internal/trojan/protocol.go` 中：

```go
package trojan

import (
    "crypto/sha256"
    "crypto/tls"
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "net"
    "strconv"
    "strings"
)

// dialTrojan 建立完整的Trojan连接
func (c *Client) dialTrojan(address string) (net.Conn, error) {
    // 1. 建立TLS连接到Trojan服务器
    tlsConfig := &tls.Config{
        ServerName:         c.config.SSL.SNI,
        InsecureSkipVerify: !c.config.SSL.Verify,
        NextProtos:         c.config.SSL.ALPN,
    }
    
    serverAddr := fmt.Sprintf("%s:%d", c.config.Server, c.config.Port)
    tlsConn, err := tls.Dial("tcp", serverAddr, tlsConfig)
    if err != nil {
        return nil, fmt.Errorf("TLS dial failed: %w", err)
    }
    
    // 2. 发送Trojan请求
    if err := c.sendTrojanRequest(tlsConn, address); err != nil {
        tlsConn.Close()
        return nil, err
    }
    
    return tlsConn, nil
}

// sendTrojanRequest 发送Trojan协议请求
func (c *Client) sendTrojanRequest(conn net.Conn, address string) error {
    // 格式：
    // +-----------------------+---------+----------------+---------+----------+
    // | hex(SHA224(password)) |  CRLF   | Trojan Request |  CRLF   |
    // +-----------------------+---------+----------------+---------+----------+
    // |          56           | X'0D0A' |    Variable    | X'0D0A' |
    // +-----------------------+---------+----------------+---------+----------+
    
    // 1. 计算密码hash
    hash := sha256.Sum224([]byte(c.config.Password))
    hexHash := hex.EncodeToString(hash[:])
    
    // 2. 构建请求
    request := []byte(hexHash + "\r\n")
    
    // 3. 添加命令 (0x01 = CONNECT)
    request = append(request, 0x01)
    
    // 4. 解析目标地址
    host, portStr, err := net.SplitHostPort(address)
    if err != nil {
        return fmt.Errorf("invalid address: %w", err)
    }
    
    port, err := strconv.Atoi(portStr)
    if err != nil {
        return fmt.Errorf("invalid port: %w", err)
    }
    
    // 5. 判断地址类型
    if ip := net.ParseIP(host); ip != nil {
        if ip.To4() != nil {
            // IPv4
            request = append(request, 0x01)
            request = append(request, ip.To4()...)
        } else {
            // IPv6
            request = append(request, 0x04)
            request = append(request, ip.To16()...)
        }
    } else {
        // 域名
        request = append(request, 0x03)
        request = append(request, byte(len(host)))
        request = append(request, []byte(host)...)
    }
    
    // 6. 添加端口 (大端序)
    portBytes := make([]byte, 2)
    binary.BigEndian.PutUint16(portBytes, uint16(port))
    request = append(request, portBytes...)
    
    // 7. 添加结束符
    request = append(request, '\r', '\n')
    
    // 8. 发送请求
    _, err = conn.Write(request)
    return err
}
```

### 步骤2：修改 client.go

```go
func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
    if c.closed {
        return nil, fmt.Errorf("client is closed")
    }
    
    // 使用完整的Trojan协议建立连接
    conn, err := c.dialTrojan(address)
    if err != nil {
        return nil, err
    }
    
    c.conn = conn
    c.lastUsed = time.Now()
    
    return conn, nil
}
```

## 快速实现指南

### 最简单的方式

1. **复制上述代码** 创建 `internal/trojan/protocol.go`
2. **修改 client.go** 的 `Dial` 方法
3. **添加必要的导入**
4. **重新编译测试**

### 验证步骤

编译后测试：

```bash
./build/proxy-server -config config.yaml &

# 测试（应该能看到不同的行为）
curl -v -x http://103.79.78.155:8080 https://www.google.com
```

**期望结果**：
- 不再是 TLS 握手错误
- 可能成功（如果Trojan服务器配置正确）
- 或有其他明确的错误（连接超时等）

## 配置检查

确保你的 `config.yaml` 配置正确：

```yaml
nodes:
  - name: "my-trojan"
    server: "xg.xgacc.top"           # ✅ Trojan服务器地址
    port: 10111                       # ✅ 端口
    password: "你的实际密码"          # ⚠️ 必须正确
    ssl:
      sni: "bilibili.com"            # ⚠️ 必须与服务器配置匹配
      verify: false
      verify_hostname: false
```

## 总结

### 当前状态
- ✅ 代理架构完整
- ✅ 请求处理正常
- ❌ Trojan协议缺失
- ❌ 无法真正代理流量

### TLS错误的原因
不是配置问题，是**协议实现缺失**导致的

### 解决方案
实现上述 `protocol.go` 中的代码，或使用 trojan-go 库

### 下一步
1. 实现Trojan协议
2. 重新编译测试
3. 验证能否访问Google

---

**这是最后也是最关键的一步！** 🚀
