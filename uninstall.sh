#!/usr/bin/env bash
# shellcheck disable=SC1091
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
cd "$SCRIPT_DIR" || exit 1
. "${SCRIPT_DIR}/scripts/common.sh"
. "${SCRIPT_DIR}/scripts/proxyctl.sh"

_valid_env || exit 1

# 停止 labproxy 进程
labproxyctl off >&/dev/null

# 移除用户级定时任务
if existing_crontab="$(crontab -l 2>/dev/null)"; then
    filtered_crontab="$(printf '%s\n' "$existing_crontab" | grep -v 'labproxyctl_auto_update' || true)"
    printf '%s\n' "$filtered_crontab" | crontab - 2>/dev/null
fi

# 清理 shell 配置
_set_rc unset

# 删除用户目录安装
rm -rf "$LABPROXY_HOME_DIR"

# 清理 labproxy 创建过的临时目录
_cleanup_labproxy_tmpdirs
if _is_labproxy_tmpdir_path "${TMPDIR:-}" || _is_labproxy_tmpdir_path "${TMP:-}" || _is_labproxy_tmpdir_path "${TEMP:-}"; then
    unset TMPDIR TMP TEMP
fi

_okcat '✨' '已卸载 labproxy 用户空间代理，相关配置已清除'
_okcat '📝' '注意：请重新加载 shell 配置或重新登录以清除环境变量'
_quit
