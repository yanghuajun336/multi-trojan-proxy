# 配置文件说明

本文档详细说明了代理服务的配置选项。完整的配置文件架构定义请参考 [contracts/config-schema.md](../specs/001-multi-trojan-proxy/contracts/config-schema.md)。

## 配置文件位置

默认配置文件路径: `config.yaml`

可通过命令行参数指定其他路径:
```bash
./proxy-server -config /path/to/config.yaml
```

## 配置示例

参考 [configs/config.example.yaml](../configs/config.example.yaml) 获取完整的配置示例。

## 核心配置项

### proxy - 代理服务配置

```yaml
proxy:
  listen: "0.0.0.0:8080"    # 监听地址和端口
  timeout: 30s              # HTTP请求超时时间
  max_concurrent: 20        # 最大并发连接数
```

### nodes - Trojan节点列表

```yaml
nodes:
  - name: "node-1"          # 节点名称（唯一标识）
    server: "example.com"   # Trojan服务器地址
    port: 443               # Trojan服务器端口
    password: "password"    # Trojan认证密码
    weight: 10              # 负载均衡权重（1-100）
    enabled: true           # 是否启用此节点
```

### health_check - 健康检查配置

```yaml
health_check:
  interval: 30s             # 检查间隔
  timeout: 5s               # 单次探测超时
  failure_threshold: 3      # 连续失败阈值
```

### logging - 日志配置

```yaml
logging:
  level: "info"             # 日志级别: debug, info, warn, error
  file: "/var/log/proxy/proxy.log"  # 日志文件路径
  max_size: 100             # 单个日志文件最大大小(MB)
```

## 路由规则（可选）

详细的路由规则配置请参考配置架构文档中的[路由规则章节](../specs/001-multi-trojan-proxy/contracts/config-schema.md#routingrule对象)。

