const assert = require("node:assert/strict");
const fs = require("node:fs");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(require.resolve("./background.js"), "utf8");

test("exposes atomic browser commands and target enumeration", async () => {
  const harness = createHarness();
  const targets = await harness.context.executeCommand("browser.targets", {});
  assert.equal(targets.agent_id, "test-agent");
  assert.equal(targets.windows[0].tabs[0].id, 21);
  const result = await harness.context.executeCommand("browser.action", { target: { tab_id: 21, window_id: 4 }, action: { id: "query", type: "dom.query", params: { selector: "main" } } });
  assert.equal(result.success, true);
  assert.equal(result.target.tab_id, 21);
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

function createHarness() {
  const tabs = new Map([[21, { id: 21, windowId: 4, index: 0, active: true, title: "Meerkit", url: "https://example.com", status: "complete", groupId: -1 }]]);
  const stats = { attached: 0, detached: 0, lastCommand: null, commands: [], cookieSetDetails: null };
  const createEvent = () => { const listeners = new Set(); return { addListener(listener) { listeners.add(listener); }, removeListener(listener) { listeners.delete(listener); }, async emit(...args) { await Promise.all([...listeners].map((listener) => listener(...args))); } }; };
  const event = createEvent();
  const debuggerDetach = createEvent();
  const chrome = {
    runtime: { getManifest: () => ({ version: "test" }), onInstalled: event, onStartup: event, onMessage: event, openOptionsPage() {} },
    alarms: { create() {}, onAlarm: event },
    storage: { local: { async get() { return {}; }, async set() {} }, session: { async get() { return {}; }, async set() {} }, onChanged: event },
    action: { async setBadgeText() {}, async setBadgeBackgroundColor() {} },
    windows: { onCreated: event, onRemoved: event, onFocusChanged: event, onBoundsChanged: event, async getAll() { return [{ id: 4, focused: true, type: "normal", state: "normal", tabs: [...tabs.values()] }]; }, async create(options) { return { id: 5, focused: true, ...options, tabs: [] }; }, async update(id, values) { return { id, focused: Boolean(values.focused), type: "normal", state: values.state || "normal", ...values }; }, async remove() {} },
    tabs: { onCreated: event, onUpdated: event, onMoved: event, onActivated: event, onAttached: event, onDetached: event, onRemoved: event, async get(id) { const value = tabs.get(id); if (!value) throw new Error("No tab"); return { ...value }; }, async query() { return [...tabs.values()]; }, async create(options) { const value = { id: 22, windowId: options.windowId || 4, index: 1, url: options.url, title: "", status: "complete" }; tabs.set(value.id, value); return { ...value }; }, async update(id, values) { Object.assign(tabs.get(id), values, { status: "complete" }); return { ...tabs.get(id) }; }, async reload() {}, async goBack() {}, async goForward() {}, async duplicate(id) { const value = { ...tabs.get(id), id: 23, index: 1 }; tabs.set(23, value); return { ...value }; }, async move(id, options) { Object.assign(tabs.get(id), { index: options.index, ...(options.windowId ? { windowId: options.windowId } : {}) }); return { ...tabs.get(id) }; }, async remove(id) { tabs.delete(id); }, async group() { return 1; }, async ungroup() {}, async setZoom() {}, async getZoom() { return 1; } },
    tabGroups: { TAB_GROUP_ID_NONE: -1, onCreated: event, onUpdated: event, onRemoved: event, async query() { return []; }, async get() { return null; }, async update() {} },
    debugger: { onEvent: event, onDetach: debuggerDetach, async attach() { stats.attached++; }, async detach() { stats.detached++; }, async sendCommand(_target, method, params) { stats.lastCommand = { method, params }; stats.commands.push({ method, params }); return method === "Page.captureScreenshot" ? { data: "AAAA" } : {}; } },
    cookies: { async getAll() { return [{ name: "session", value: "secret", domain: "example.com", path: "/", secure: true, httpOnly: true, sameSite: "lax", storeId: "0" }]; }, async set(details) { stats.cookieSetDetails = details; return { ...details, storeId: "0" }; }, async remove(details) { return details; } },
    scripting: { async executeScript(options) { if (options.func?.name === "elementCenter") return [{ result: { x: 100, y: 80 } }]; if (options.func?.name === "queryElements") return [{ result: { total: 3, elements: [{ text: "one" }, { text: "two" }], truncated: true } }]; if (options.func?.name === "getWebStorage") return [{ result: { area: options.args[0], count: 1, values: { token: "value" }, truncated: false } }]; if (options.func?.name === "setWebStorage") return [{ result: { area: options.args[0], key: options.args[1], written: true, size: options.args[2].length } }]; return [{ result: { text: "ok" } }]; } }
  };
  const context = vm.createContext({ chrome, console, crypto: { randomUUID: () => "test-agent" }, performance, setInterval, clearInterval, setTimeout, clearTimeout, TextEncoder, URL, WebSocket: class { static OPEN = 1; } });
  vm.runInContext(source, context, { filename: "background.js" });
  return { context, stats, events: { debuggerDetach } };
}
