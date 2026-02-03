#!/bin/bash

# 配置文件验证脚本
# 用于验证 YAML 配置文件的语法

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${YELLOW}ℹ${NC} $1"
}

# 检查 yq 是否安装
check_yq() {
    if ! command -v yq &> /dev/null; then
        print_error "yq 未安装，请先安装 yq"
        echo "安装方法："
        echo "  Ubuntu/Debian: sudo snap install yq"
        echo "  macOS: brew install yq"
        echo "  或访问: https://github.com/mikefarah/yq"
        exit 1
    fi
}

# 验证 YAML 文件语法
validate_yaml() {
    local file=$1
    
    if [ ! -f "$file" ]; then
        print_error "文件不存在: $file"
        return 1
    fi
    
    print_info "验证文件: $file"
    
    if yq eval '.' "$file" > /dev/null 2>&1; then
        print_success "语法正确"
        return 0
    else
        print_error "语法错误"
        yq eval '.' "$file"
        return 1
    fi
}

# 主函数
main() {
    check_yq
    
    local config_dir="configs"
    local has_error=0
    
    echo "================================"
    echo "配置文件语法验证"
    echo "================================"
    echo
    
    # 验证所有示例配置文件
    for file in "$config_dir"/*.example.yaml; do
        if [ -f "$file" ]; then
            validate_yaml "$file" || has_error=1
            echo
        fi
    done
    
    # 验证用户配置文件（如果存在）
    if [ -f "$config_dir/config.yaml" ]; then
        validate_yaml "$config_dir/config.yaml" || has_error=1
        echo
    fi
    
    if [ -f "$config_dir/rules.yaml" ]; then
        validate_yaml "$config_dir/rules.yaml" || has_error=1
        echo
    fi
    
    echo "================================"
    if [ $has_error -eq 0 ]; then
        print_success "所有配置文件验证通过"
        exit 0
    else
        print_error "部分配置文件存在错误"
        exit 1
    fi
}

main "$@"
