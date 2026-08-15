(function initializeDebugController(scope) {
  const storageKey = "debugModeTabIds";
  const inspectorFile = "content/selector-inspector.js";

  function create(chromeAPI) {
    const enabledTabs = new Set();
    let ready = null;

    function initialize() {
      if (ready) return ready;
      ready = chromeAPI.storage.session.get(storageKey).then((stored) => {
        for (const value of stored[storageKey] || []) if (Number.isInteger(Number(value))) enabledTabs.add(Number(value));
      }).catch(() => {});
      chromeAPI.tabs.onRemoved.addListener((tabId) => { if (enabledTabs.delete(tabId)) void persist(); });
      chromeAPI.tabs.onUpdated.addListener((tabId, changeInfo) => {
        if (changeInfo.status === "complete" && enabledTabs.has(tabId)) void inject(tabId).catch(() => disable(tabId));
      });
      return ready;
    }

    async function persist() {
      await chromeAPI.storage.session.set({ [storageKey]: [...enabledTabs] });
    }

    async function activeTab() {
      const [tab] = await chromeAPI.tabs.query({ active: true, currentWindow: true });
      return tab || null;
    }

    function inspectable(tab) {
      return Boolean(tab?.id && /^(https?|file):/i.test(tab.url || ""));
    }

    async function inject(tabId) {
      await chromeAPI.scripting.executeScript({ target: { tabId }, files: [inspectorFile] });
    }

    async function enable(tab) {
      if (!inspectable(tab)) throw new Error("当前页面不支持元素调试，请打开普通网页后重试。");
      await inject(tab.id);
      enabledTabs.add(tab.id);
      await persist();
      return tab;
    }

    async function disable(tabId) {
      enabledTabs.delete(tabId);
      await chromeAPI.tabs.sendMessage?.(tabId, { type: "meerkit.selector-inspector", enabled: false }).catch(() => {});
      await persist();
    }

    async function status() {
      await initialize();
      const tab = await activeTab();
      return { enabled: Boolean(tab?.id && enabledTabs.has(tab.id)), supported: inspectable(tab), tabId: tab?.id || 0, title: tab?.title || "", url: tab?.url || "" };
    }

    async function handleMessage(message, sender) {
      await initialize();
      if (message.type === "debug.status") return status();
      if (message.type === "debug.set") {
        const tab = message.tabId ? await chromeAPI.tabs.get(Number(message.tabId)) : await activeTab();
        if (!tab?.id) throw new Error("没有可调试的活动标签页。");
        if (message.enabled) await enable(tab); else await disable(tab.id);
        return status();
      }
      if (message.type === "debug.inspector.disabled" && sender?.tab?.id) {
        await disable(sender.tab.id);
        return { enabled: false };
      }
      return null;
    }

    return { initialize, handleMessage, status };
  }

  scope.MeerkitDebugController = Object.freeze({ create });
})(globalThis);
