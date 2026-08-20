const assert = require("node:assert/strict");
const fs = require("node:fs");
const test = require("node:test");
const vm = require("node:vm");

const source = ["modules/config.js", "modules/action-badge.js", "modules/debug-controller.js", "background.js"].map((file) => fs.readFileSync(require.resolve(`./${file}`), "utf8")).join("\n");

test("exposes atomic browser commands and target enumeration", async () => {
  const harness = createHarness();
  const targets = await harness.context.executeCommand("browser.targets", {});
  assert.equal(targets.agent_id, "test-agent");
  assert.equal(targets.windows[0].tabs[0].id, 21);
  const result = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { id: "query", type: "dom.query", params: { selector: "main" } } });
  assert.equal(result.success, true);
  assert.equal(result.target.tab_id, 21);
});

test("returns bounded selector candidates for a target tab", async () => {
  const harness = createHarness();
  const result = await harness.context.executeCommand("browser.selector_candidates", { target: { tab_id: 21, window_id: 4 }, queries: ["button", "a[href]"], limit: 20 });
  assert.equal(result.items[0].selector, "#save");
  assert.equal(result.items[0].tag_name, "button");
  assert.equal(result.total, 1);
});

test("selector candidate injection is self-contained", () => {
  const harness = createHarness();
  const source = harness.context.collectSelectorCandidates.toString();
  for (const externalName of ["selectorForElement", "selectorIsUnique", "escapeCSSIdentifier", "escapeCSSString"]) {
    assert.equal(source.includes(externalName), false, `${externalName} must be defined inside the injected function`);
  }
});

test("DOM value injections are self-contained", () => {
  const harness = createHarness();
  for (const name of ["inputElement", "selectElement"]) {
    const sandbox = createDOMControlSandbox();
    const injected = vm.runInNewContext(`(${harness.context[name].toString()})`, sandbox);
    const result = injected("#control", "updated");
    assert.equal(sandbox.control.value, "updated", `${name} must set the native control value`);
    assert.equal(result.selector, "#control");
    if (name === "inputElement") {
      assert.equal(result.focused, true);
      assert.equal(result.updated, true);
    }
    assert.deepEqual(sandbox.control.events, ["input", "change"]);
  }
});

test("runs DOM control mutations in the page main world", async () => {
  const harness = createHarness();
  await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "dom.input", params: { selector: "input", value: "updated" } } });
  await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "dom.check", params: { selector: "input", checked: true } } });
  await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "dom.select", params: { selector: "select", value: "updated" } } });
  assert.deepEqual(harness.stats.scriptWorlds, ["MAIN", "MAIN", "MAIN"]);
});

test("rejects a tab selected from another window", async () => {
  const harness = createHarness();
  await assert.rejects(() => harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 9 }, action: { type: "page.wait", params: { mode: "load" } } }), /selected window/);
});

test("detaches active network captures when the Meerkit connection closes", async () => {
  const harness = createHarness();
  await harness.context.startNetworkSession({ session_id: "capture-1", target: { tab_id: 21, window_id: 4 }, rules: [{ id: "api" }] });
  assert.equal(harness.stats.attached, 1);
  await harness.context.stopAllNetworkSessions("connection closed");
  assert.equal(harness.stats.detached, 1);
});

test("chunks large command responses without losing payload data", async () => {
  const harness = createHarness();
  harness.context.sentMessages = [];
  vm.runInContext("socket = { readyState: WebSocket.OPEN, bufferedAmount: 0, send(value) { sentMessages.push(JSON.parse(value)); } }", harness.context);
  const result = { type: "page.screenshot", success: true, data: { data_url: `data:image/png;base64,${"A".repeat(1200000)}` } };
  await harness.context.sendCommandResult("request-1", result);
  const messages = harness.context.sentMessages;
  assert.ok(messages.length > 1);
  assert.ok(messages.every((message) => message.type === "response_chunk" && message.id === "request-1"));
  assert.deepEqual(JSON.parse(messages.map((message) => message.chunk).join("")), result);
});

test("captures full-page WebP screenshots with bounded image metadata", async () => {
  const harness = createHarness();
  const result = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { id: "shot", type: "page.screenshot", params: { format: "webp", quality: 72, full_page: true } } });
  assert.equal(result.data.data_url, "data:image/webp;base64,AAAA");
  assert.equal(result.data.format, "webp");
  assert.equal(result.data.full_page, true);
  assert.equal(harness.stats.lastCommand.method, "Page.captureScreenshot");
  assert.equal(JSON.stringify(harness.stats.lastCommand.params), JSON.stringify({ format: "webp", quality: 72, captureBeyondViewport: true }));
});

test("executes window and tab state actions", async () => {
  const harness = createHarness();
  const windowResult = await harness.context.executeCommand("browser.action", { target: { window_id: 4 }, action: { type: "window.state", params: { state: "maximized" } } });
  assert.equal(windowResult.data.state, "maximized");
  const tabResult = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "tab.pin", params: { pinned: true } } });
  assert.equal(tabResult.data.pinned, true);
  const defaultPin = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "tab.pin", params: {} } });
  assert.equal(defaultPin.data.pinned, true);
  const unpinned = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "tab.pin", params: { pinned: false } } });
  assert.equal(unpinned.data.pinned, false);
});

test("executes tab resource management actions", async () => {
  const harness = createHarness();
  const discard = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "tab.discard", params: {} } });
  assert.equal(discard.data.discarded, true);
  const automatic = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "tab.auto_discardable", params: { auto_discardable: false } } });
  assert.equal(automatic.data.auto_discardable, false);
  const language = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "tab.detect_language", params: {} } });
  assert.equal(language.data.language, "en");
});

test("stops loading and returns page performance", async () => {
  const harness = createHarness();
  const stopped = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "page.stop_loading", params: {} } });
  assert.equal(stopped.data.stopped, true);
  assert.equal(harness.stats.lastCommand.method, "Page.stopLoading");
  const performanceResult = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "page.performance", params: {} } });
  assert.equal(performanceResult.data.resources.count, 4);
});

test("executes bounded DOM mutation actions", async () => {
  const harness = createHarness();
  const attribute = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "dom.set_attribute", params: { selector: "main", name: "data-state", value: "ready" } } });
  assert.equal(attribute.data.value, "ready");
  const event = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "dom.dispatch_event", params: { selector: "input", event: "change" } } });
  assert.equal(event.data.event, "change");
  assert.equal(event.data.bubbles, true);
});

test("executes cookie and web storage actions", async () => {
  const harness = createHarness();
  const cookies = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "cookie.list", params: {} } });
  assert.equal(cookies.data.cookies[0].name, "session");
  const changed = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "cookie.set", params: { name: "session", value: "updated", same_site: "lax" } } });
  assert.equal(changed.data.value, "updated");
  assert.equal(harness.stats.cookieSetDetails.sameSite, "lax");
  await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "cookie.set", params: { name: "session", value: "default" } } });
  assert.equal("sameSite" in harness.stats.cookieSetDetails, false);
  const storage = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "storage.get", params: { area: "local" } } });
  assert.equal(storage.data.area, "local");
  const written = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "storage.set", params: { area: "local", key: "token", value: "updated" } } });
  assert.equal(written.data.written, true);
});

test("returns bounded DOM collections", async () => {
  const harness = createHarness();
  const result = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "dom.query_all", params: { selector: "li", limit: 2, max_length: 64 } } });
  assert.equal(result.data.elements.length, 2);
  assert.equal(result.data.truncated, true);
});

test("bounds web storage by final UTF-8 JSON size", () => {
  const harness = createHarness();
  harness.context.localStorage = { length: 1, key: () => "quoted", getItem: () => `值${"\\\"".repeat(500)}` };
  const result = harness.context.getWebStorage("local", "", 1024, 200);
  assert.equal(result.truncated, true);
  assert.ok(Buffer.byteLength(JSON.stringify(result), "utf8") <= 200);
});

test("shares one debugger attachment between capture and screenshot", async () => {
  const harness = createHarness();
  const session = await harness.context.startNetworkSession({ session_id: "capture-shared", target: { tab_id: 21, window_id: 4 }, rules: [{ id: "api" }] });
  await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "page.screenshot", params: {} } });
  await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "input.click", params: { selector: "button" } } });
  assert.equal(harness.stats.attached, 1);
  assert.equal(harness.stats.detached, 0);
  await harness.context.stopNetworkSession({ session_id: session.id });
  assert.equal(harness.stats.detached, 1);
});

test("keeps cache disabled until the last requesting capture stops", async () => {
  const harness = createHarness();
  const first = await harness.context.startNetworkSession({ session_id: "capture-cache-1", target: { tab_id: 21, window_id: 4 }, disable_cache: true, rules: [{ id: "all" }] });
  const second = await harness.context.startNetworkSession({ session_id: "capture-cache-2", target: { tab_id: 21, window_id: 4 }, disable_cache: true, rules: [{ id: "all" }] });
  assert.equal(harness.stats.commands.filter((item) => item.method === "Network.setCacheDisabled" && item.params.cacheDisabled).length, 1);
  await harness.context.stopNetworkSession({ session_id: first.id });
  assert.equal(harness.stats.commands.filter((item) => item.method === "Network.setCacheDisabled" && !item.params.cacheDisabled).length, 0);
  await harness.context.stopNetworkSession({ session_id: second.id });
  assert.equal(harness.stats.commands.filter((item) => item.method === "Network.setCacheDisabled" && !item.params.cacheDisabled).length, 1);
});

test("enables and disables selector inspection for the active tab", async () => {
  const harness = createHarness();
  const controller = harness.context.MeerkitDebugController.create(harness.context.chrome);
  await controller.initialize();
  const enabled = await controller.handleMessage({ type: "debug.set", enabled: true });
  assert.equal(enabled.enabled, true);
  assert.ok(harness.stats.scriptFiles.includes("content/selector-inspector.js"));
  const disabled = await controller.handleMessage({ type: "debug.set", enabled: false });
  assert.equal(disabled.enabled, false);
  assert.equal(harness.stats.tabMessages.at(-1).message.enabled, false);
});

test("shows active task count in the extension badge", () => {
  const harness = createHarness();
  const badge = harness.context.MeerkitActionBadge.create(harness.context.chrome);
  badge.update("connected", 3);
  assert.equal(harness.stats.badgeTexts.at(-1), "3");
  assert.equal(harness.stats.badgeColors.at(-1), "#2563eb");
  badge.update("connected", 0);
  assert.equal(harness.stats.badgeTexts.at(-1), "0");
  badge.update("failed", 0);
  assert.equal(harness.stats.badgeTexts.at(-1), "·");
  assert.equal(harness.stats.badgeColors.at(-1), "#d4d4d8");
  badge.update("idle", 0);
  assert.equal(harness.stats.badgeTexts.at(-1), "");
});

test("connects only after user action and stops heartbeat on disconnect", async () => {
  const harness = createHarness();
  await vm.runInContext("connectionReady", harness.context);
  assert.equal(vm.runInContext("connectionState", harness.context), "idle");
  assert.equal(harness.stats.sockets.length, 0);
  assert.equal(harness.stats.badgeTexts.at(-1), "");

  await harness.context.chrome.storage.local.set({ pairingToken: "pairing-token" });
  await harness.context.requestConnect();
  assert.equal(harness.stats.sockets.length, 1);
  assert.equal(vm.runInContext("connectionState", harness.context), "connecting");
  assert.equal(vm.runInContext("heartbeatTimer", harness.context), null);

  const socket = harness.stats.sockets[0];
  await socket.emit("open");
  assert.equal(socket.sent[0].type, "hello");
  await harness.context.handleMessage(JSON.stringify({ protocol: 1, type: "welcome", payload: { heartbeat_seconds: 30 } }));
  assert.equal(vm.runInContext("connectionState", harness.context), "connected");
  assert.notEqual(vm.runInContext("heartbeatTimer", harness.context), null);

  await harness.context.requestDisconnect();
  assert.equal(vm.runInContext("connectionState", harness.context), "idle");
  assert.equal(vm.runInContext("heartbeatTimer", harness.context), null);
  assert.equal(harness.stats.badgeTexts.at(-1), "");
  await harness.events.alarm.emit({ name: "meerkit-connection-retry" });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(harness.stats.sockets.length, 1);
});

test("retries automatically after an unexpected disconnect", async () => {
  const harness = createHarness();
  await vm.runInContext("connectionReady", harness.context);
  await harness.context.chrome.storage.local.set({ pairingToken: "pairing-token" });
  await harness.context.requestConnect();
  const socket = harness.stats.sockets[0];
  await socket.emit("open");
  await harness.context.handleMessage(JSON.stringify({ protocol: 1, type: "welcome", payload: {} }));

  await socket.emit("close");
  assert.equal(vm.runInContext("connectionState", harness.context), "failed");
  assert.equal(vm.runInContext("connectionEnabled", harness.context), true);
  assert.equal(harness.stats.sockets.length, 1);
  assert.equal(harness.stats.badgeTexts.at(-1), "·");
  assert.equal(harness.stats.badgeColors.at(-1), "#d4d4d8");

  await harness.events.alarm.emit({ name: "meerkit-connection-retry" });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(harness.stats.sockets.length, 2);
  assert.equal(vm.runInContext("connectionState", harness.context), "connecting");
  await harness.stats.sockets[1].emit("close");
  assert.equal(vm.runInContext("connectionEnabled", harness.context), true);
  await harness.events.alarm.emit({ name: "meerkit-connection-retry" });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(harness.stats.sockets.length, 3);
  await harness.context.requestDisconnect();
});

test("keeps retry enabled and reports a rejected handshake", async () => {
  const harness = createHarness();
  await vm.runInContext("connectionReady", harness.context);
  await harness.context.chrome.storage.local.set({ pairingToken: "stale-token" });
  await harness.context.requestConnect();
  const socket = harness.stats.sockets[0];
  await socket.emit("open");
  await harness.context.handleMessage(JSON.stringify({ protocol: 1, type: "error", error: "browser extension pairing failed" }));
  assert.equal(vm.runInContext("connectionEnabled", harness.context), true);
  assert.equal(vm.runInContext("connectionState", harness.context), "failed");
  assert.equal(vm.runInContext("lastError", harness.context), "browser extension pairing failed");
  await harness.context.requestDisconnect();
});

test("stops network capture when Chrome detaches the shared debugger", async () => {
  const harness = createHarness();
  await harness.context.startNetworkSession({ session_id: "capture-detached", target: { tab_id: 21, window_id: 4 }, rules: [{ id: "api" }] });
  await harness.events.debuggerDetach.emit({ tabId: 21 }, "replaced_with_devtools");
  await new Promise((resolve) => setImmediate(resolve));
  await assert.rejects(() => harness.context.stopNetworkSession({ session_id: "capture-detached" }), /not found/);
});

test("dispatches real input through CDP", async () => {
  const harness = createHarness();
  await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { type: "input.click", params: { selector: "button", click_count: 2 } } });
  assert.equal(harness.stats.commands.filter((item) => item.method === "Input.dispatchMouseEvent").length, 3);
  assert.equal(harness.stats.attached, 1);
  assert.equal(harness.stats.detached, 1);
});

function createDOMControlSandbox() {
  const sandboxDocument = { activeElement: null, querySelector: () => control };
  class HTMLElement {
    constructor() { this.events = []; this.isContentEditable = false; }
    focus() { this.focused = true; sandboxDocument.activeElement = this; }
    dispatchEvent(event) { this.events.push(event.type); }
  }
  class HTMLInputElement extends HTMLElement {}
  class HTMLTextAreaElement extends HTMLElement {}
  class HTMLSelectElement extends HTMLElement {
    constructor() { super(); this.options = [{ value: "updated" }]; }
  }
  for (const prototype of [HTMLInputElement.prototype, HTMLTextAreaElement.prototype, HTMLSelectElement.prototype]) {
    Object.defineProperty(prototype, "value", {
      configurable: true,
      get() { return this.nativeValue || ""; },
      set(value) { this.nativeValue = value; }
    });
  }
  class Event { constructor(type) { this.type = type; } }
  const control = new HTMLSelectElement();
  return {
    control,
    document: sandboxDocument,
    Event,
    HTMLElement,
    HTMLInputElement,
    HTMLTextAreaElement,
    HTMLSelectElement
  };
}

function createHarness() {
  const tabs = new Map([[21, { id: 21, windowId: 4, index: 0, active: true, title: "Meerkit", url: "https://example.com", status: "complete", groupId: -1 }]]);
  const stats = { attached: 0, detached: 0, lastCommand: null, commands: [], cookieSetDetails: null, scriptWorlds: [], scriptFiles: [], tabMessages: [], badgeTexts: [], badgeColors: [], sockets: [], alarms: new Map(), localStorage: {} };
  const sessionStorage = {};
  const createEvent = () => { const listeners = new Set(); return { addListener(listener) { listeners.add(listener); }, removeListener(listener) { listeners.delete(listener); }, async emit(...args) { await Promise.all([...listeners].map((listener) => listener(...args))); } }; };
  const event = createEvent();
  const debuggerDetach = createEvent();
  const alarm = createEvent();
  const chrome = {
    runtime: { getManifest: () => ({ version: "test" }), onInstalled: event, onStartup: event, onMessage: event, openOptionsPage() {} },
    alarms: { create(name, options) { stats.alarms.set(name, options); }, async clear(name) { return stats.alarms.delete(name); }, onAlarm: alarm },
    storage: { local: { async get(keys) { if (typeof keys === "string") return { [keys]: stats.localStorage[keys] }; if (keys && typeof keys === "object") return { ...keys, ...stats.localStorage }; return { ...stats.localStorage }; }, async set(values) { Object.assign(stats.localStorage, values); } }, session: { async get(key) { return typeof key === "string" ? { [key]: sessionStorage[key] } : { ...sessionStorage }; }, async set(values) { Object.assign(sessionStorage, values); } }, onChanged: event },
    action: { async setBadgeText({ text }) { stats.badgeTexts.push(text); }, async setBadgeBackgroundColor({ color }) { stats.badgeColors.push(color); } },
    windows: { onCreated: event, onRemoved: event, onFocusChanged: event, onBoundsChanged: event, async getAll() { return [{ id: 4, focused: true, type: "normal", state: "normal", tabs: [...tabs.values()] }]; }, async create(options) { return { id: 5, focused: true, ...options, tabs: [] }; }, async update(id, values) { return { id, focused: Boolean(values.focused), type: "normal", state: values.state || "normal", ...values }; }, async remove() {} },
    tabs: { onCreated: event, onUpdated: event, onMoved: event, onActivated: event, onAttached: event, onDetached: event, onRemoved: event, async get(id) { const value = tabs.get(id); if (!value) throw new Error("No tab"); return { ...value }; }, async query() { return [...tabs.values()]; }, async sendMessage(tabId, message) { stats.tabMessages.push({ tabId, message }); return { ok: true }; }, async create(options) { const value = { id: 22, windowId: options.windowId || 4, index: 1, url: options.url, title: "", status: "complete" }; tabs.set(value.id, value); return { ...value }; }, async update(id, values) { Object.assign(tabs.get(id), values, { status: "complete" }); return { ...tabs.get(id) }; }, async reload() {}, async goBack() {}, async goForward() {}, async duplicate(id) { const value = { ...tabs.get(id), id: 23, index: 1 }; tabs.set(23, value); return { ...value }; }, async move(id, options) { Object.assign(tabs.get(id), { index: options.index, ...(options.windowId ? { windowId: options.windowId } : {}) }); return { ...tabs.get(id) }; }, async discard(id) { Object.assign(tabs.get(id), { discarded: true }); return { ...tabs.get(id) }; }, async detectLanguage() { return "en"; }, async remove(id) { tabs.delete(id); }, async group() { return 1; }, async ungroup() {}, async setZoom() {}, async getZoom() { return 1; } },
    tabGroups: { TAB_GROUP_ID_NONE: -1, onCreated: event, onUpdated: event, onRemoved: event, async query() { return []; }, async get() { return null; }, async update() {} },
    debugger: { onEvent: event, onDetach: debuggerDetach, async attach() { stats.attached++; }, async detach() { stats.detached++; }, async sendCommand(_target, method, params) { stats.lastCommand = { method, params }; stats.commands.push({ method, params }); return method === "Page.captureScreenshot" ? { data: "AAAA" } : {}; } },
    cookies: { async getAll() { return [{ name: "session", value: "secret", domain: "example.com", path: "/", secure: true, httpOnly: true, sameSite: "lax", storeId: "0" }]; }, async set(details) { stats.cookieSetDetails = details; return { ...details, storeId: "0" }; }, async remove(details) { return details; } },
    scripting: { async executeScript(options) { if (options.files) { stats.scriptFiles.push(...options.files); return []; } stats.scriptWorlds.push(options.world); if (options.func?.name === "elementCenter") return [{ result: { x: 100, y: 80 } }]; if (options.func?.name === "collectSelectorCandidates") return [{ result: { items: [{ selector: "#save", tag_name: "button", text: "Save", visible: true, unique: true }], total: 1, truncated: false } }]; if (options.func?.name === "queryElements") return [{ result: { total: 3, elements: [{ text: "one" }, { text: "two" }], truncated: true } }]; if (options.func?.name === "performanceSnapshot") return [{ result: { resources: { count: 4 } } }]; if (options.func?.name === "setElementAttribute") return [{ result: { selector: options.args[0], name: options.args[1], value: options.args[2] } }]; if (options.func?.name === "dispatchElementEvent") return [{ result: { selector: options.args[0], event: options.args[1], bubbles: options.args[2], cancelable: options.args[3], default_prevented: false } }]; if (options.func?.name === "getWebStorage") return [{ result: { area: options.args[0], count: 1, values: { token: "value" }, truncated: false } }]; if (options.func?.name === "setWebStorage") return [{ result: { area: options.args[0], key: options.args[1], written: true, size: options.args[2].length } }]; return [{ result: { text: "ok" } }]; } }
  };
  class FakeWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSED = 3;
    constructor(url) { this.url = url; this.readyState = FakeWebSocket.CONNECTING; this.bufferedAmount = 0; this.listeners = new Map(); this.sent = []; stats.sockets.push(this); }
    addEventListener(type, listener) { const listeners = this.listeners.get(type) || []; listeners.push(listener); this.listeners.set(type, listeners); }
    async emit(type, event = {}) { if (type === "open") this.readyState = FakeWebSocket.OPEN; if (type === "close") this.readyState = FakeWebSocket.CLOSED; await Promise.all((this.listeners.get(type) || []).map((listener) => listener(event))); }
    send(value) { this.sent.push(JSON.parse(value)); }
    close(code, reason) { this.readyState = FakeWebSocket.CLOSED; void this.emit("close", { code, reason }); }
  }
  const context = vm.createContext({ chrome, console, crypto: { randomUUID: () => "test-agent" }, performance, setInterval, clearInterval, setTimeout, clearTimeout, TextEncoder, URL, importScripts() {}, WebSocket: FakeWebSocket });
  vm.runInContext(source, context, { filename: "background.js" });
  return { context, stats, events: { alarm, debuggerDetach } };
}
