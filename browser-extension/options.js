const defaults = MeerkitConfig.defaultSettings;
const form = document.querySelector("#settings-form");
const endpoint = document.querySelector("#endpoint");
const token = document.querySelector("#pairing-token");
const name = document.querySelector("#agent-name");
const concurrency = document.querySelector("#max-concurrent");
const statusElement = document.querySelector("#status");
const connectionToggle = document.querySelector("#connection-toggle");
let connectionEnabled = false;

async function load() {
  const values = { ...defaults, ...(await chrome.storage.local.get(defaults)) };
  endpoint.value = values.endpoint;
  token.value = values.pairingToken;
  name.value = values.agentName;
  concurrency.value = values.maxConcurrent;
  await refreshStatus();
}

async function refreshStatus() {
  const value = await chrome.runtime.sendMessage({ type: "status" }).catch(() => ({ state: "idle", enabled: false, error: "扩展后台不可用" }));
  connectionEnabled = Boolean(value.enabled);
  statusElement.dataset.state = value.state;
  statusElement.querySelector("strong").textContent = value.state === "connected" ? "已连接 Meerkit" : value.state === "connecting" ? "正在连接" : value.state === "failed" ? `连接失败：${value.error || "无法连接 Meerkit"}` : value.state === "unconfigured" ? value.error || "等待配置" : "未连接";
  connectionToggle.textContent = connectionEnabled ? "断开连接" : value.state === "failed" ? "重新连接" : "连接后端";
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  await chrome.storage.local.set({ endpoint: endpoint.value.trim(), pairingToken: token.value.trim(), agentName: name.value.trim(), maxConcurrent: Number(concurrency.value) });
  await refreshStatus();
});
connectionToggle.addEventListener("click", async () => {
  await chrome.runtime.sendMessage({ type: connectionEnabled ? "connection.disconnect" : "connection.connect" });
  await refreshStatus();
});
document.querySelector("#toggle-token").addEventListener("click", () => { const visible = token.type === "text"; token.type = visible ? "password" : "text"; document.querySelector("#toggle-token").textContent = visible ? "显示" : "隐藏"; });
setInterval(refreshStatus, 2000);
void load();
