# Multi-Trojan Proxy Service

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/yanghuajun336/multi-trojan-proxy?style=social)](https://github.com/yanghuajun336/multi-trojan-proxy/stargazers)

统一的HTTP代理服务，内部管理多个Trojan客户端连接，提供单一代理入口，支持自动故障转移和负载均衡。

## 功能特性

- 🚀 **统一代理入口**: 单一HTTP代理地址，简化客户端配置
- 🔄 **自动故障转移**: 节点故障时自动切换，确保服务连续性
- ⚖️ **负载均衡**: 支持加权轮询，充分利用多节点资源
- 💪 **高可用性**: 定期健康检查，智能节点选择
- 👥 **多用户支持**: 支持多客户端并发连接
- 🔧 **热重载**: 配置文件修改后自动生效，无需重启
- 🎯 **智能路由**: 支持基于域名、IP、地理位置的路由规则

## 快速开始

详细的安装和配置指南请查看 [docs/quickstart.md](docs/quickstart.md)

### 基本步骤

1. 编译程序:
   ```bash
   go build -o proxy-server ./cmd/proxy
   ```

2. 创建配置文件:
   ```bash
   cp configs/config.example.yaml config.yaml
   # 编辑 config.yaml 配置你的Trojan节点
   ```

3. 启动服务:
   ```bash
   ./proxy-server -config config.yaml
   ```

4. 配置客户端使用代理:
   ```
   HTTP代理: your_server_ip:8080
   HTTPS代理: your_server_ip:8080
   ```

## 项目结构

```
proxy/
├── cmd/proxy/          # 主程序入口
├── internal/           # 内部包
│   ├── config/        # 配置管理
│   ├── proxy/         # HTTP代理服务器
│   ├── router/        # 路由规则引擎
│   ├── trojan/        # Trojan节点管理
│   └── health/        # 健康检查
├── pkg/logger/        # 日志工具
├── configs/           # 配置文件示例
├── docs/              # 文档
└── tests/             # 测试
```

## 配置说明

配置文件使用YAML格式，主要包含以下部分:

- **proxy**: 代理服务配置（监听地址、超时等）
- **nodes**: Trojan节点列表
- **routing**: 路由规则（可选）
- **health_check**: 健康检查配置
- **logging**: 日志配置

完整配置说明请参考 [docs/configuration.md](docs/configuration.md)

## 系统要求

- Go 1.21+
- Linux系统（Ubuntu 20.04+ 推荐）
- 至少一个可用的Trojan服务器

## 技术栈

- **语言**: Go 1.21+
- **Trojan客户端**: [trojan-go](https://github.com/p4gefau1t/trojan-go)
- **配置解析**: yaml.v3
- **文件监听**: fsnotify
- **GeoIP查询**: geoip2-golang

## License

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 文档

- [快速开始指南](docs/quickstart.md)
- [配置文件说明](docs/configuration.md)
- [技术架构](specs/001-multi-trojan-proxy/plan.md)
- [数据模型](specs/001-multi-trojan-proxy/data-model.md)
