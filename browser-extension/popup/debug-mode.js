export function initializeDebugMode() {
  const toggle = document.querySelector("#debug-mode");
  const copy = document.querySelector("#debug-description");
  const tab = document.querySelector("#debug-tab");
  let updating = false;
  let errorMessage = "";

  async function render() {
    const value = await chrome.runtime.sendMessage({ type: "debug.status" }).catch((error) => ({ enabled: false, supported: false, error: error?.message || "调试服务不可用" }));
    toggle.checked = Boolean(value.enabled);
    toggle.disabled = updating || !value.supported;
    tab.textContent = value.title || value.url || "当前页面不可调试";
    tab.title = value.url || "";
    copy.textContent = errorMessage || value.error || (value.enabled ? "已开启：单击页面元素固定目标，然后复制；也可按 Ctrl/⌘ + C。" : value.supported ? "开启后悬浮预览，单击元素固定后可复制选择器。" : "Chrome 内部页面和扩展页面不支持元素调试。");
    document.querySelector("#popup-debug").dataset.enabled = value.enabled ? "true" : "false";
  }

  toggle.addEventListener("change", async () => {
    updating = true; toggle.disabled = true;
    try {
      const result = await chrome.runtime.sendMessage({ type: "debug.set", enabled: toggle.checked });
      if (result?.error) throw new Error(result.error);
      errorMessage = "";
    } catch (error) {
      errorMessage = error?.message || "无法切换调试模式";
    } finally { updating = false; await render(); }
  });
  void render();
}
