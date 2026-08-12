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

function createHarness() {
  const tabs = new Map([[21, { id: 21, windowId: 4, index: 0, active: true, title: "Meerkit", url: "https://example.com", status: "complete", groupId: -1 }]]);
  const stats = { attached: 0, detached: 0 };
  const event = { addListener() {}, removeListener() {} };
  const chrome = {
    runtime: { getManifest: () => ({ version: "test" }), onInstalled: event, onStartup: event, onMessage: event, openOptionsPage() {} },
    alarms: { create() {}, onAlarm: event },
    storage: { local: { async get() { return {}; }, async set() {} }, session: { async get() { return {}; }, async set() {} }, onChanged: event },
    action: { async setBadgeText() {}, async setBadgeBackgroundColor() {} },
    windows: { onCreated: event, onRemoved: event, onFocusChanged: event, async getAll() { return [{ id: 4, focused: true, type: "normal", tabs: [...tabs.values()] }]; } },
    tabs: { onCreated: event, onUpdated: event, onMoved: event, onAttached: event, onDetached: event, onRemoved: event, async get(id) { const value = tabs.get(id); if (!value) throw new Error("No tab"); return { ...value }; }, async query() { return [...tabs.values()]; }, async create(options) { const value = { id: 22, windowId: options.windowId || 4, url: options.url, title: "", status: "complete" }; tabs.set(value.id, value); return { ...value }; }, async update(id, values) { Object.assign(tabs.get(id), values, { status: "complete" }); return { ...tabs.get(id) }; }, async remove(id) { tabs.delete(id); }, async group() { return 1; } },
    tabGroups: { TAB_GROUP_ID_NONE: -1, onCreated: event, onUpdated: event, onRemoved: event, async query() { return []; }, async get() { return null; }, async update() {} },
    debugger: { onEvent: event, async attach() { stats.attached++; }, async detach() { stats.detached++; }, async sendCommand() { return {}; } },
    scripting: { async executeScript() { return [{ result: { text: "ok" } }]; } }
  };
  const context = vm.createContext({ chrome, console, crypto: { randomUUID: () => "test-agent" }, performance, setInterval, clearInterval, setTimeout, clearTimeout, URL, WebSocket: class {} });
  vm.runInContext(source, context, { filename: "background.js" });
  return { context, stats };
}
