# 配置文件分离说明

## 概述

为了避免配置文件过长，现在支持将路由规则分离到独立的配置文件中。

## 使用方法

### 方式一：使用分离配置（推荐）

1. **创建主配置文件** `config.yaml`：

```yaml
proxy:
  listen: "0.0.0.0:8080"
  timeout: 30s
  max_concurrent: 20

nodes:
  - name: "us-node-1"
    server: "us1.trojan.example.com"
    port: 443
    password: "your_password_here"
    weight: 10
    enabled: true

routing:
  rules_file: "rules.yaml"  # 引用外部规则文件

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "info"
  file: "/var/log/proxy/proxy.log"
  max_size: 100
```

2. **创建规则文件** `rules.yaml`：

```yaml
geoip_database: "data/GeoLite2-Country.mmdb"

rules:
  # 直连规则
  - type: DOMAIN-SUFFIX
    pattern: "baidu.com"
    action: DIRECT
    
  # 代理规则
  - type: DOMAIN-SUFFIX
    pattern: "google.com"
    action: PROXY
    
  # 更多规则...
  
  # 默认规则（必须放在最后）
  - type: FINAL
    action: PROXY
```

### 方式二：使用内联配置（向后兼容）

你仍然可以在主配置文件中直接定义所有规则：

```yaml
proxy:
  listen: "0.0.0.0:8080"
  timeout: 30s
  max_concurrent: 20

nodes:
  - name: "us-node-1"
    server: "us1.trojan.example.com"
    port: 443
    password: "your_password_here"
    weight: 10
    enabled: true

routing:
  geoip_database: "data/GeoLite2-Country.mmdb"
  rules:
    - type: DOMAIN-SUFFIX
      pattern: "baidu.com"
      action: DIRECT
    - type: FINAL
      action: PROXY

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "info"
  file: "/var/log/proxy/proxy.log"
  max_size: 100
```

## 配置项说明

### routing.rules_file

- **类型**: 字符串
- **可选**: 是
- **说明**: 外部规则文件路径
- **路径处理**:
  - 相对路径：相对于主配置文件所在目录
  - 绝对路径：直接使用指定的绝对路径

### GeoIP 数据库优先级

当使用分离配置时，GeoIP 数据库路径可以在两个地方配置：

1. **主配置文件** (`routing.geoip_database`) - 优先级高
2. **规则文件** (`geoip_database`) - 优先级低

如果主配置文件中指定了 `geoip_database`，将使用主配置的值；否则使用规则文件中的值。

## 示例文件

项目提供了以下示例文件：

- `configs/config.example.yaml` - 内联配置示例（向后兼容）
- `configs/config.split.example.yaml` - 分离配置示例
- `configs/rules.example.yaml` - 规则文件示例

## 优势

### 使用分离配置的好处：

1. **可维护性**: 主配置文件更简洁，专注于服务配置
2. **可复用性**: 规则文件可以在多个配置中共享
3. **易于管理**: 规则变更不影响主配置
4. **版本控制**: 可以独立跟踪规则文件的变更历史

### 向后兼容性：

- 现有的内联配置仍然完全支持
- 无需修改现有配置文件即可继续使用
- 可以根据需要逐步迁移到分离配置

## 测试

运行测试以验证配置加载功能：

```bash
go test ./internal/config/... -v
```

## 注意事项

1. 如果同时指定了 `rules_file` 和内联的 `rules`，外部文件的规则会覆盖内联规则
2. 规则文件必须存在且格式正确，否则配置加载会失败
3. 规则文件路径建议使用相对路径，便于配置文件的移植
