export function initializeStatus() {
  const status = document.querySelector("#popup-status");
  const activeRuns = document.querySelector("#active-runs");

  async function render() {
    const value = await chrome.runtime.sendMessage({ type: "status" }).catch(() => ({ state: "disconnected", error: "后台服务不可用" }));
    status.dataset.state = value.state;
    status.querySelector("strong").textContent = value.state === "connected" ? "已连接" : value.state === "connecting" ? "正在连接" : value.error || "未连接";
    document.querySelector("#agent-name").textContent = value.agentName || "Meerkit";
    document.querySelector("#endpoint").textContent = value.endpoint || "尚未配置";
    document.querySelector("#version").textContent = value.version || "-";
    activeRuns.textContent = String(value.activeRuns || 0);
    activeRuns.dataset.active = value.activeRuns > 0 ? "true" : "false";
  }

  document.querySelector("#reconnect").addEventListener("click", async () => { await chrome.runtime.sendMessage({ type: "reconnect" }); setTimeout(render, 300); });
  const timer = setInterval(render, 750);
  addEventListener("unload", () => clearInterval(timer), { once: true });
  void render();
}
