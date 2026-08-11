async function render() {
  const value = await chrome.runtime.sendMessage({ type: "status" }).catch(() => ({ state: "disconnected", error: "后台服务不可用" }));
  const status = document.querySelector("#popup-status");
  status.dataset.state = value.state;
  status.querySelector("strong").textContent = value.state === "connected" ? "已连接" : value.state === "connecting" ? "正在连接" : value.error || "未连接";
  document.querySelector("#agent-name").textContent = value.agentName || "Meerkit";
  document.querySelector("#endpoint").textContent = value.endpoint || "尚未配置";
  document.querySelector("#version").textContent = value.version || "-";
  document.querySelector("#active-runs").textContent = String(value.activeRuns || 0);
}
document.querySelector("#settings").addEventListener("click", () => chrome.runtime.openOptionsPage());
document.querySelector("#reconnect").addEventListener("click", async () => { await chrome.runtime.sendMessage({ type: "reconnect" }); setTimeout(render, 300); });
void render();
