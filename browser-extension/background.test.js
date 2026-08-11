const assert = require("node:assert/strict");
const fs = require("node:fs");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(require.resolve("./background.js"), "utf8");

test("reuses a redirected tab in the current and restored browser session", async () => {
  const localState = {};
  const first = createHarness({ localState, redirects: { "https://example.com/start": "https://example.com/signed-in" } });

  const initial = await first.context.executeRun(reusableRequest());
  assert.equal(initial.actions[0].data.reused, false);
  assert.equal(initial.actions[0].data.url, "https://example.com/signed-in");
  assert.equal(first.created.length, 1);
  assert.equal(first.tabs.size, 1);

  const repeated = await first.context.executeRun(reusableRequest());
  assert.equal(repeated.actions[0].data.reused, true);
  assert.equal(repeated.actions[0].data.url, "https://example.com/signed-in");
  assert.deepEqual(first.reloaded, [1]);
  assert.equal(first.created.length, 1);
  assert.equal(first.groups.size, 1);

  const restartedWorker = createHarness({
    localState,
    tabs: [{ id: 1, windowId: 1, groupId: 5, url: "https://example.com/account", status: "complete" }],
    groups: [{ id: 5, windowId: 1, title: "Meerkit", color: "blue", collapsed: false }]
  });
  const afterWorkerRestart = await restartedWorker.context.executeRun(reusableRequest());
  assert.equal(afterWorkerRestart.actions[0].data.reused, true);
  assert.deepEqual(restartedWorker.reloaded, [1]);
  assert.equal(restartedWorker.created.length, 0);
  assert.equal(restartedWorker.groups.size, 1);

  const restored = createHarness({ localState, tabs: [{ id: 9, windowId: 1, url: "https://example.com/account", status: "complete" }] });
  const afterRestart = await restored.context.executeRun(reusableRequest());
  assert.equal(afterRestart.actions[0].data.reused, true);
  assert.deepEqual(restored.reloaded, [9]);
  assert.equal(restored.created.length, 0);
});

test("serializes concurrent runs that use the same reusable tab", async () => {
  const harness = createHarness();
  const [first, second] = await Promise.all([
    harness.context.executeRun(reusableRequest()),
    harness.context.executeRun(reusableRequest())
  ]);
  assert.equal(first.actions[0].data.reused, false);
  assert.equal(second.actions[0].data.reused, true);
  assert.equal(harness.created.length, 1);
  assert.equal(harness.groups.size, 1);
});

test("adopts a tab recorded by the previous URL-only storage format", async () => {
  const localState = { reusableTabURLs: { "https://example.com/start": "https://example.com/signed-in" } };
  const harness = createHarness({ localState, tabs: [{ id: 4, windowId: 1, url: "https://example.com/signed-in", status: "complete" }] });
  const result = await harness.context.executeRun(reusableRequest());
  assert.equal(result.actions[0].data.reused, true);
  assert.equal(harness.created.length, 0);
  assert.equal(localState["reusableTab:browser-example-html:https://example.com/start"].tab_id, 4);
});

test("creates a new tab in the window of an existing Meerkit group", async () => {
  const harness = createHarness({ groups: [{ id: 3, windowId: 2, title: "Meerkit", color: "blue", collapsed: false }] });
  const result = await harness.context.executeRun(reusableRequest());
  assert.equal(harness.tabs.get(result.tab_id).windowId, 2);
  assert.equal(result.actions[1].data.group_id, 3);
  assert.equal(result.actions[1].data.reused, true);
  assert.equal(harness.groups.size, 1);
});

test("always-new mode closes only the tab created for that run", async () => {
  const harness = createHarness({ tabs: [{ id: 7, windowId: 1, url: "https://example.com/existing", status: "complete" }] });
  const request = reusableRequest();
  request.keep_tab = false;
  request.actions[0].params.reuse = false;

  const result = await harness.context.executeRun(request);
  assert.equal(result.actions[0].data.reused, false);
  assert.equal(harness.tabs.has(7), true);
  assert.equal(harness.tabs.has(result.tab_id), false);
});

test("recreates a manually closed tab and keeps the same reuse key", async () => {
  const localState = {};
  const harness = createHarness({ localState });
  const first = await harness.context.executeRun(reusableRequest());
  const storageKey = "reusableTab:browser-example-html:https://example.com/start";

  await harness.chrome.tabs.remove(first.tab_id);
  const second = await harness.context.executeRun(reusableRequest());

  assert.equal(second.actions[0].data.reused, false);
  assert.notEqual(second.tab_id, first.tab_id);
  assert.equal(harness.tabs.has(second.tab_id), true);
  assert.equal(localState[storageKey].tab_id, second.tab_id);
  assert.equal(harness.created.length, 2);
});

test("changing the reuse key preserves the old tab and creates a separate association", async () => {
  const localState = {};
  const harness = createHarness({ localState });
  const first = await harness.context.executeRun(reusableRequest());
  const changed = reusableRequest();
  changed.actions[0].params.reuse_key = "browser-example-html:account-b";

  const second = await harness.context.executeRun(changed);

  assert.equal(second.actions[0].data.reused, false);
  assert.notEqual(second.tab_id, first.tab_id);
  assert.equal(harness.tabs.has(first.tab_id), true);
  assert.equal(harness.tabs.has(second.tab_id), true);
  assert.equal(localState["reusableTab:browser-example-html:https://example.com/start"].tab_id, first.tab_id);
  assert.equal(localState["reusableTab:browser-example-html:account-b"].tab_id, second.tab_id);
});

function reusableRequest() {
  return {
    keep_tab: true,
    actions: [
      { id: "open", type: "tab.open", params: { url: "https://example.com/start", active: false, reuse: true, reuse_key: "browser-example-html:https://example.com/start", group_title: "Meerkit" } },
      { id: "group", type: "tab.group", params: { title: "Meerkit", color: "blue", reuse_group: true } }
    ]
  };
}

function createHarness({ localState = {}, tabs = [], groups: initialGroups = [], redirects = {} } = {}) {
  const tabMap = new Map(tabs.map((tab) => [tab.id, { title: "", ...tab }]));
  const sessionState = {};
  const groups = new Map(initialGroups.map((group) => [group.id, { ...group }]));
  const created = [];
  const reloaded = [];
  let nextTabID = Math.max(0, ...tabMap.keys()) + 1;
  let nextGroupID = Math.max(0, ...groups.keys()) + 1;
  const event = { addListener() {}, removeListener() {} };
  const storageArea = (state) => ({
    async get(query) {
      if (typeof query === "string") return { [query]: state[query] };
      if (Array.isArray(query)) return Object.fromEntries(query.map((key) => [key, state[key]]));
      return { ...(query || {}), ...state };
    },
    async set(values) { Object.assign(state, values); }
  });
  const chrome = {
    runtime: { getManifest: () => ({ version: "test" }), onInstalled: event, onStartup: event, onMessage: event, openOptionsPage() {} },
    alarms: { create() {}, onAlarm: event },
    storage: { local: storageArea(localState), session: storageArea(sessionState), onChanged: event },
    action: { async setBadgeText() {}, async setBadgeBackgroundColor() {} },
    tabs: {
      onUpdated: event,
      onRemoved: event,
      async create(options) {
        const tab = { id: nextTabID++, windowId: options.windowId || 1, url: options.url, title: "", status: "complete" };
        tabMap.set(tab.id, tab);
        created.push(tab.id);
        return { ...tab };
      },
      async get(id) {
        const tab = tabMap.get(id);
        if (!tab) throw new Error("No tab");
        return { ...tab };
      },
      async query() { return [...tabMap.values()].map((tab) => ({ ...tab })); },
      async update(id, values) {
        const tab = tabMap.get(id);
        Object.assign(tab, values);
        if (values.url) tab.url = redirects[values.url] || values.url;
        tab.status = "complete";
        return { ...tab };
      },
      async reload(id) { reloaded.push(id); },
      async remove(id) { tabMap.delete(id); },
      async group(options) {
        const groupID = options.groupId || nextGroupID++;
        const tab = tabMap.get(options.tabIds[0]);
        tab.groupId = groupID;
        return groupID;
      }
    },
    tabGroups: {
      TAB_GROUP_ID_NONE: -1,
      async query({ windowId } = {}) { return [...groups.values()].filter((group) => windowId == null || group.windowId === windowId).map((group) => ({ ...group })); },
      async get(id) {
        const group = groups.get(id);
        if (!group) throw new Error("No group");
        return { ...group };
      },
      async update(id, values) {
        groups.set(id, { id, windowId: 1, ...(groups.get(id) || {}), ...values });
        return groups.get(id);
      }
    },
    debugger: { onEvent: event },
    scripting: {}
  };
  const context = vm.createContext({ chrome, console, crypto: { randomUUID: () => "test-agent" }, performance, setInterval, clearInterval, setTimeout, clearTimeout, URL, WebSocket: class {} });
  vm.runInContext(source, context, { filename: "background.js" });
  return { context, chrome, tabs: tabMap, groups, created, reloaded };
}
