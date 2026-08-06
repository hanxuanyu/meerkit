export function pluginStatusMeta(status) {
  return ({
    healthy: { label: "运行中", tone: "success" },
    installed: { label: "未启用", tone: "muted" },
    disabled: { label: "已禁用", tone: "muted" },
    degraded: { label: "异常", tone: "danger" },
    starting: { label: "启动中", tone: "warning" },
    updating: { label: "更新中", tone: "warning" },
  })[status] || { label: status || "未知", tone: "muted" };
}

export function pluginTrustMeta(trustState) {
  return ({
    development: { label: "开发源码", tone: "warning" },
    official: { label: "官方可信", tone: "success" },
    trusted: { label: "已验证", tone: "success" },
    untrusted: { label: "待信任", tone: "warning" },
    unsigned: { label: "未签名", tone: "muted" },
  })[trustState] || { label: trustState || "未知", tone: "muted" };
}
