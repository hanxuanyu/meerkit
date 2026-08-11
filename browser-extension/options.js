const defaults = { endpoint: "ws://127.0.0.1:8080/api/v1/browser/extension/ws", pairingToken: "", agentName: "Local Chrome", maxConcurrent: 2 };
const form = document.querySelector("#settings-form");
const endpoint = document.querySelector("#endpoint");
const token = document.querySelector("#pairing-token");
const name = document.querySelector("#agent-name");
const concurrency = document.querySelector("#max-concurrent");
const statusElement = document.querySelector("#status");

async function load() {
  const values = { ...defaults, ...(await chrome.storage.local.get(defaults)) };
  endpoint.value = values.endpoint;
  token.value = values.pairingToken;
  name.value = values.agentName;
  concurrency.value = values.maxConcurrent;
  await refreshStatus();
}

async function refreshStatus() {
  const value = await chrome.runtime.sendMessage({ type: "status" }).catch(() => ({ state: "disconnected", error: "后台服务不可用" }));
  statusElement.dataset.state = value.state;
  statusElement.querySelector("strong").textContent = value.state === "connected" ? "已连接 Meerkit" : value.state === "connecting" ? "正在连接" : value.state === "unconfigured" ? "等待配置" : value.error || "尚未连接";
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  await chrome.storage.local.set({ endpoint: endpoint.value.trim(), pairingToken: token.value.trim(), agentName: name.value.trim(), maxConcurrent: Number(concurrency.value) });
  await chrome.runtime.sendMessage({ type: "reconnect" });
  await refreshStatus();
});
document.querySelector("#reconnect").addEventListener("click", async () => { await chrome.runtime.sendMessage({ type: "reconnect" }); setTimeout(refreshStatus, 400); });
document.querySelector("#toggle-token").addEventListener("click", () => { const visible = token.type === "text"; token.type = visible ? "password" : "text"; document.querySelector("#toggle-token").textContent = visible ? "显示" : "隐藏"; });
setInterval(refreshStatus, 2000);
void load();
