#!/usr/bin/env bash
# shellcheck disable=SC1091
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
cd "$SCRIPT_DIR" || exit 1
. "${SCRIPT_DIR}/scripts/common.sh"
. "${SCRIPT_DIR}/scripts/proxyctl.sh"

# 用于检查环境是否有效
_valid_env || exit 1

if [ -d "$LABPROXY_HOME_DIR" ]; then
    _error_quit "请先执行卸载脚本以清除安装路径：${LABPROXY_HOME_DIR}"
fi

_get_kernel

# 创建用户目录结构
mkdir -p "$LABPROXY_HOME_DIR"/{bin,config,logs}

# 解压并安装二进制文件到用户目录
if ! gzip -dc "$ZIP_KERNEL" > "${LABPROXY_HOME_DIR}/bin/$BIN_KERNEL_NAME"; then
    _error_quit "解压内核文件失败：${ZIP_KERNEL}"
fi
chmod +x "${LABPROXY_HOME_DIR}/bin/$BIN_KERNEL_NAME"

if ! tar -xf "$ZIP_SUBCONVERTER" -C "${LABPROXY_HOME_DIR}/bin"; then
    _error_quit "解压 subconverter 失败：${ZIP_SUBCONVERTER}"
fi

if ! tar -xf "$ZIP_YQ" -C "${LABPROXY_HOME_DIR}/bin"; then
    _error_quit "解压 yq 失败：${ZIP_YQ}"
fi

# 重命名 yq 二进制文件（yq_linux_amd64 -> yq）
for yq_file in "${LABPROXY_HOME_DIR}/bin"/yq_*; do
    if [ -f "$yq_file" ]; then
        mv "$yq_file" "${LABPROXY_HOME_DIR}/bin/yq"
        break
    fi
done
chmod +x "${LABPROXY_HOME_DIR}/bin/yq"

# 设置二进制文件路径
_set_bin

# 验证或获取配置文件
url=""
if ! _valid_config "$RESOURCES_CONFIG"; then
    echo -n "$(_okcat '🔗' '输入订阅地址：')"
    read -r url
    _okcat '⏳' '正在下载...'

    if ! _download_config "$RESOURCES_CONFIG" "$url"; then
        _error_quit "下载失败，请将配置内容写入 ${RESOURCES_CONFIG} 后重新安装"
    fi

    if ! _valid_config "$RESOURCES_CONFIG"; then
        _error_quit "配置校验失败：${RESOURCES_CONFIG}，转换日志：${BIN_SUBCONVERTER_LOG}"
    fi
fi
_okcat '✅' '配置校验通过'

if [ -n "$url" ]; then
    echo "$url" > "$LABPROXY_CONFIG_URL"
fi

cp -rf "$SCRIPT_BASE_DIR" "$LABPROXY_HOME_DIR/"
cp "$RESOURCES_BASE_DIR"/*.yaml "$LABPROXY_HOME_DIR/" 2>/dev/null || true
cp "$RESOURCES_BASE_DIR"/*.mmdb "$LABPROXY_HOME_DIR/" 2>/dev/null || true
cp "$RESOURCES_BASE_DIR"/*.dat "$LABPROXY_HOME_DIR/" 2>/dev/null || true

# 优先使用预编译的 TUI，若能力不足则自动回退到源码构建
if _get_tui_archive && [ -n "$ZIP_LABPROXY_TUI" ]; then
    _okcat '⏳' '解压预编译 TUI...'
    mkdir -p "$(dirname "$LABPROXY_TUI_BIN")"
    if tar -xzf "$ZIP_LABPROXY_TUI" -C "$(dirname "$LABPROXY_TUI_BIN")"; then
        # 重命名二进制文件（去掉架构后缀）
        tui_bin_name=$(basename "$ZIP_LABPROXY_TUI" .tar.gz)
        if [ -f "$(dirname "$LABPROXY_TUI_BIN")/$tui_bin_name" ]; then
            mv "$(dirname "$LABPROXY_TUI_BIN")/$tui_bin_name" "$LABPROXY_TUI_BIN"
        fi
        chmod +x "$LABPROXY_TUI_BIN"
        if _tui_supports_restart_command "$LABPROXY_TUI_BIN"; then
            _okcat '✅' '预编译 TUI 安装完成'
        else
            _failcat '⚠️' '预编译 TUI 版本较旧，回退到源码构建'
            _install_tui_from_source
        fi
    else
        _failcat '⚠️' '解压预编译 TUI 失败，回退到源码构建'
        _install_tui_from_source
    fi
else
    _install_tui_from_source
fi

# 解压 zashboard UI
if ! unzip -q -o "$ZIP_UI" -d "$LABPROXY_HOME_DIR"; then
    _error_quit "解压 UI 文件失败：${ZIP_UI}"
fi
mv "${LABPROXY_HOME_DIR}/dist" "${LABPROXY_HOME_DIR}/ui"

# 设置 shell 配置
_set_rc

# 启动代理服务（会自动合并配置和检查端口冲突）
labproxyctl on

# 显示 Web UI 信息（启动后显示实际端口）
labproxyui

_okcat '🎉' 'LabProxy 用户空间代理已安装完成！'
_okcat '📋' '使用说明：'
_okcat '💡' '命令前缀：labproxy | labproxyctl'
_okcat '  • 开启/关闭：labproxy on/off'
_okcat '  • 重启服务：labproxy restart'
_okcat '  • 查看状态：labproxy status'
_okcat '  • Web 控制台：labproxy ui'
_okcat '  • TUI 控制台：labproxy tui'
_okcat '  • 更新订阅：labproxy update [auto|log]'
_okcat '  • 设置订阅：labproxy subscribe [URL]'
_okcat '  • 系统代理：labproxy proxy [on|off|status]'
_okcat '  • 局域网访问：labproxy lan [on|off|status]'
_okcat ''
_okcat '📂' "安装目录：${LABPROXY_HOME_DIR}"
_okcat '📂' "配置目录：${LABPROXY_HOME_DIR}/config/"
_okcat '📂' "日志目录：${LABPROXY_HOME_DIR}/logs/"

_quit
