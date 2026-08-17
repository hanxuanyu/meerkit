export function initializeStatus() {
  const status = document.querySelector("#popup-status");
  const activeRuns = document.querySelector("#active-runs");
  const connectionToggle = document.querySelector("#connection-toggle");
  let connectionEnabled = false;

  async function render() {
    const value = await chrome.runtime.sendMessage({ type: "status" }).catch(() => ({ state: "idle", enabled: false, error: "扩展后台不可用" }));
    connectionEnabled = Boolean(value.enabled);
    status.dataset.state = value.state;
    status.querySelector("strong").textContent = value.state === "connected" ? "已连接" : value.state === "connecting" ? "正在连接" : value.state === "failed" ? "连接失败" : value.state === "unconfigured" ? "等待配置" : "未连接";
    document.querySelector("#agent-name").textContent = value.agentName || "Meerkit";
    document.querySelector("#endpoint").textContent = value.error || value.endpoint || "尚未配置";
    document.querySelector("#version").textContent = value.version || "-";
    activeRuns.textContent = String(value.activeRuns || 0);
    activeRuns.dataset.active = value.activeRuns > 0 ? "true" : "false";
    connectionToggle.textContent = connectionEnabled ? "断开连接" : value.state === "failed" ? "重新连接" : "连接后端";
  }

  connectionToggle.addEventListener("click", async () => {
    await chrome.runtime.sendMessage({ type: connectionEnabled ? "connection.disconnect" : "connection.connect" });
    await render();
  });
  const timer = setInterval(render, 750);
  addEventListener("unload", () => clearInterval(timer), { once: true });
  void render();
}
