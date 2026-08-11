const PROTOCOL_VERSION = 1;
const EXTENSION_VERSION = chrome.runtime.getManifest().version;
const DEFAULT_SETTINGS = {
  endpoint: "ws://127.0.0.1:8080/api/v1/browser/extension/ws",
  pairingToken: "",
  agentName: "Local Chrome",
  maxConcurrent: 2
};
const CAPABILITIES = [
  "tab.open", "tab.navigate", "tab.close", "tab.group",
  "page.wait", "page.screenshot",
  "dom.document", "dom.query", "dom.click", "dom.input",
  "runtime.evaluate", "network.capture"
];

let socket = null;
let reconnectTimer = null;
let heartbeatTimer = null;
let reconnectAttempt = 0;
let connectionState = "disconnected";
let lastError = "";
let activeRuns = 0;
const leasedTabIds = new Set();
const leasedReuseKeys = new Set();

chrome.runtime.onInstalled.addListener(() => {
  void ensureIdentity();
  chrome.runtime.openOptionsPage();
  scheduleReconnect(100);
});
chrome.runtime.onStartup.addListener(() => scheduleReconnect(100));
chrome.alarms.create("meerkit-browser-reconnect", { periodInMinutes: 1 });
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "meerkit-browser-reconnect" && (!socket || socket.readyState > WebSocket.OPEN)) connect();
});
chrome.storage.onChanged.addListener((changes, area) => {
  if (area === "local" && ["endpoint", "pairingToken", "agentName", "maxConcurrent"].some((key) => changes[key])) reconnect();
});
chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type === "status") {
    void status().then(sendResponse);
    return true;
  }
  if (message?.type === "reconnect") {
    reconnect();
    sendResponse({ ok: true });
  }
  return false;
});

async function settings() {
  return { ...DEFAULT_SETTINGS, ...(await chrome.storage.local.get(DEFAULT_SETTINGS)) };
}

async function ensureIdentity() {
  const stored = await chrome.storage.local.get("agentId");
  if (stored.agentId) return stored.agentId;
  const agentId = crypto.randomUUID();
  await chrome.storage.local.set({ agentId });
  return agentId;
}

async function status() {
  const config = await settings();
  return { state: connectionState, error: lastError, endpoint: config.endpoint, agentName: config.agentName, activeRuns, version: EXTENSION_VERSION };
}

function setState(state, error = "") {
  connectionState = state;
  lastError = error;
  void chrome.action.setBadgeText({ text: state === "connected" ? "" : "!" });
  void chrome.action.setBadgeBackgroundColor({ color: state === "connected" ? "#16a34a" : "#dc2626" });
}

async function connect() {
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) return;
  clearTimeout(reconnectTimer);
  const config = await settings();
  if (!config.endpoint || !config.pairingToken) {
    setState("unconfigured", "Configure the Meerkit WebSocket endpoint and pairing token.");
    return;
  }
  setState("connecting");
  try {
    socket = new WebSocket(config.endpoint);
  } catch (error) {
    setState("disconnected", error.message);
    scheduleReconnect();
    return;
  }
  socket.addEventListener("open", async () => {
    const agentId = await ensureIdentity();
    send({
      protocol: PROTOCOL_VERSION,
      type: "hello",
      token: config.pairingToken,
      payload: { id: agentId, name: config.agentName, version: EXTENSION_VERSION, capabilities: CAPABILITIES }
    });
  });
  socket.addEventListener("message", (event) => void handleMessage(event.data));
  socket.addEventListener("close", () => {
    stopHeartbeat();
    socket = null;
    if (connectionState !== "unconfigured") setState("disconnected", lastError || "Connection closed.");
    scheduleReconnect();
  });
  socket.addEventListener("error", () => setState("disconnected", "Unable to connect to Meerkit."));
}

function reconnect() {
  stopHeartbeat();
  reconnectAttempt = 0;
  if (socket) socket.close(1000, "configuration changed");
  socket = null;
  scheduleReconnect(100);
}

function scheduleReconnect(delay) {
  clearTimeout(reconnectTimer);
  const wait = delay ?? Math.min(30000, 1000 * (2 ** reconnectAttempt++));
  reconnectTimer = setTimeout(connect, wait);
}

function startHeartbeat(seconds = 15) {
  stopHeartbeat();
  heartbeatTimer = setInterval(() => send({ protocol: PROTOCOL_VERSION, type: "ping" }), Math.max(10, seconds) * 1000);
}

function stopHeartbeat() {
  clearInterval(heartbeatTimer);
  heartbeatTimer = null;
}

function send(message) {
  if (!socket || socket.readyState !== WebSocket.OPEN) return false;
  socket.send(JSON.stringify(message));
  return true;
}

async function handleMessage(raw) {
  let message;
  try { message = JSON.parse(raw); } catch { return; }
  if (message.protocol !== PROTOCOL_VERSION) {
    lastError = `Unsupported protocol version ${message.protocol}.`;
    socket?.close(1002, "protocol mismatch");
    return;
  }
  if (message.type === "welcome") {
    reconnectAttempt = 0;
    setState("connected");
    startHeartbeat(message.payload?.heartbeat_seconds || 15);
    return;
  }
  if (message.type === "pong") return;
  if (message.type !== "command" || message.command !== "browser.run" || !message.id) return;
  const config = await settings();
  if (activeRuns >= Math.max(1, Number(config.maxConcurrent) || 1)) {
    send({ protocol: PROTOCOL_VERSION, type: "response", id: message.id, error: "browser agent concurrency limit reached" });
    return;
  }
  activeRuns++;
  try {
    const result = await executeRun(message.payload || {});
    send({ protocol: PROTOCOL_VERSION, type: "response", id: message.id, payload: result });
  } catch (error) {
    send({ protocol: PROTOCOL_VERSION, type: "response", id: message.id, error: safeError(error) });
  } finally {
    activeRuns--;
  }
}

async function executeRun(request) {
  const started = performance.now();
  const timeoutMS = clamp(Number(request.timeout_ms) || 60000, 1000, 300000);
  const deadline = Date.now() + timeoutMS;
  const state = { tabId: null, createdTab: false, reuseKey: "", debuggerAttached: false, capture: null };
  const actionResults = [];
  try {
    for (const action of request.actions || []) {
      const actionStarted = performance.now();
      try {
        const data = await withTimeout(executeAction(state, action, request.network_captures || [], deadline), remaining(deadline), `Action ${action.id || action.type} timed out.`);
        actionResults.push({ id: action.id, type: action.type, success: true, duration_ms: Math.round(performance.now() - actionStarted), data: data || {} });
      } catch (error) {
        actionResults.push({ id: action.id, type: action.type, success: false, duration_ms: Math.round(performance.now() - actionStarted), error: safeError(error) });
        if (!action.continue_on_error) throw error;
      }
    }
    if (state.capture) await state.capture.flush(deadline);
    return {
      agent_id: await ensureIdentity(),
      tab_id: state.tabId || undefined,
      duration_ms: Math.round(performance.now() - started),
      actions: actionResults,
      network: state.capture?.results || []
    };
  } finally {
    if (state.capture) await state.capture.stop();
    if (state.tabId) leasedTabIds.delete(state.tabId);
    if (state.reuseKey) leasedReuseKeys.delete(state.reuseKey);
    if (state.tabId && state.createdTab && !request.keep_tab) await chrome.tabs.remove(state.tabId).catch(() => {});
  }
}

async function executeAction(state, action, captureRules, deadline) {
  const params = action.params || {};
  switch (action.type) {
    case "tab.open": {
      if (state.tabId) throw new Error("A browser tab is already active for this run; close it before opening another tab.");
      validateURL(params.url || "about:blank", true);
      const reuseKey = String(params.reuse_key || params.url || "");
      if (params.reuse && reuseKey) {
        await acquireReuseKey(reuseKey, deadline);
        state.reuseKey = reuseKey;
      }
      let tab = params.reuse ? await findReusableTab(reuseKey, params.url) : null;
      state.createdTab = !tab;
      if (!tab) {
        const group = params.group_title ? await findTabGroup(undefined, String(params.group_title)) : null;
        tab = await chrome.tabs.create({ url: "about:blank", active: params.active !== false, ...(group ? { windowId: group.windowId } : {}) });
      }
      state.tabId = tab.id;
      leasedTabIds.add(tab.id);
      state.capture = null;
      state.debuggerAttached = false;
      if (captureRules.length) {
        state.capture = await createNetworkCapture(tab.id, captureRules);
        state.debuggerAttached = true;
      }
      if (!state.createdTab) {
        await chrome.tabs.reload(tab.id);
        if (params.wait !== false) await waitForTab(tab.id, deadline);
      } else if (params.url && params.url !== "about:blank") {
        await chrome.tabs.update(tab.id, { url: params.url });
        if (params.wait !== false) await waitForTab(tab.id, deadline);
      }
      const finalTab = await chrome.tabs.get(tab.id);
      if (params.reuse && reuseKey) await rememberReusableTab(reuseKey, finalTab, String(params.group_title || ""));
      return { ...tabInfo(finalTab), reused: !state.createdTab };
    }
    case "tab.navigate": {
      requireTab(state);
      validateURL(params.url);
      await chrome.tabs.update(state.tabId, { url: params.url });
      if (params.wait !== false) await waitForTab(state.tabId, deadline);
      return tabInfo(await chrome.tabs.get(state.tabId));
    }
    case "tab.group": {
      requireTab(state);
      const title = String(params.title || "Meerkit");
      const current = await chrome.tabs.get(state.tabId);
      if (current.groupId != null && current.groupId !== chrome.tabGroups.TAB_GROUP_ID_NONE) {
        const currentGroup = await chrome.tabGroups.get(current.groupId).catch(() => null);
        if (currentGroup?.title === title) return { group_id: currentGroup.id, reused: true };
      }
      const existing = params.reuse_group ? await findTabGroup(current.windowId, title) : null;
      const groupId = await chrome.tabs.group({ tabIds: [state.tabId], ...(existing ? { groupId: existing.id } : {}) });
      await chrome.tabGroups.update(groupId, { title: String(params.title || "Meerkit"), color: validGroupColor(params.color), collapsed: Boolean(params.collapsed) });
      return { group_id: groupId, reused: Boolean(existing) };
    }
    case "tab.close": {
      requireTab(state);
      const tabId = state.tabId;
      if (state.capture) await state.capture.stop();
      await chrome.tabs.remove(tabId);
      state.tabId = null;
      leasedTabIds.delete(tabId);
      if (state.reuseKey) leasedReuseKeys.delete(state.reuseKey);
      state.createdTab = false;
      state.reuseKey = "";
      state.capture = null;
      state.debuggerAttached = false;
      return { tab_id: tabId };
    }
    case "page.wait": {
      requireTab(state);
      if (params.selector) await waitForSelector(state.tabId, String(params.selector), clamp(Number(params.timeout_ms) || remaining(deadline), 100, remaining(deadline)));
      else if (params.duration_ms) await sleep(clamp(Number(params.duration_ms), 0, remaining(deadline)));
      else await waitForTab(state.tabId, deadline);
      return { ready: true };
    }
    case "page.screenshot": {
      requireTab(state);
      const format = params.format === "jpeg" ? "jpeg" : "png";
      const target = { tabId: state.tabId };
      let attachedHere = false;
      if (!state.debuggerAttached) {
        await chrome.debugger.attach(target, "1.3");
        attachedHere = true;
      }
      try {
        const screenshot = await chrome.debugger.sendCommand(target, "Page.captureScreenshot", {
          format,
          ...(format === "jpeg" ? { quality: clamp(Number(params.quality) || 90, 1, 100) } : {}),
          captureBeyondViewport: Boolean(params.full_page)
        });
        return { data_url: `data:image/${format};base64,${screenshot.data}` };
      } finally {
        if (attachedHere) await chrome.debugger.detach(target).catch(() => {});
      }
    }
    case "dom.document":
      requireTab(state);
      return runInTab(state.tabId, documentSnapshot, [clamp(Number(params.max_length) || 262144, 1024, 1048576)]);
    case "dom.query":
      requireTab(state);
      return runInTab(state.tabId, queryElement, [requiredSelector(params.selector), clamp(Number(params.max_length) || 65536, 256, 1048576)]);
    case "dom.click":
      requireTab(state);
      return runInTab(state.tabId, clickElement, [requiredSelector(params.selector)]);
    case "dom.input":
      requireTab(state);
      return runInTab(state.tabId, inputElement, [requiredSelector(params.selector), String(params.value ?? "")]);
    case "runtime.evaluate": {
      requireTab(state);
      const expression = String(params.expression || "");
      if (!expression || expression.length > 100000) throw new Error("A JavaScript expression between 1 and 100000 characters is required.");
      return runInMainWorld(state.tabId, evaluateExpression, [expression]);
    }
    default:
      throw new Error(`Unsupported browser action: ${action.type}`);
  }
}

async function acquireReuseKey(reuseKey, deadline) {
  while (leasedReuseKeys.has(reuseKey)) {
    if (remaining(deadline) <= 1) throw new Error(`Timed out waiting for reusable tab ${reuseKey}.`);
    await sleep(Math.min(100, remaining(deadline)));
  }
  leasedReuseKeys.add(reuseKey);
}

async function findReusableTab(reuseKey, targetURL) {
  const storageKey = reusableTabStorageKey(reuseKey);
  const [sessionState, localState] = await Promise.all([
    chrome.storage.session.get([storageKey, "reusableTabs"]),
    chrome.storage.local.get(null)
  ]);
  const sessionTabId = sessionState[storageKey]?.tab_id || sessionState.reusableTabs?.[reuseKey];
  if (sessionTabId && !leasedTabIds.has(sessionTabId)) {
    const tab = await chrome.tabs.get(sessionTabId).catch(() => null);
    if (tab && !leasedTabIds.has(tab.id)) {
      leasedTabIds.add(tab.id);
      return tab;
    }
  }

  const record = localState[storageKey] || {};
  const legacyFinalURL = localState.reusableTabURLs?.[reuseKey] || localState.reusableTabURLs?.[targetURL];
  const candidates = new Set([record.final_url, legacyFinalURL, targetURL].map(comparableURL).filter(Boolean));
  if (record.tab_id && !leasedTabIds.has(record.tab_id)) {
    const tab = await chrome.tabs.get(record.tab_id).catch(() => null);
    if (tab && !leasedTabIds.has(tab.id) && (candidates.has(comparableURL(tab.url)) || await tabBelongsToGroup(tab, record.group_title))) {
      leasedTabIds.add(tab.id);
      return tab;
    }
  }
  if (!candidates.size) return null;
  const ownedByOtherKeys = new Set(Object.entries(localState)
    .filter(([key, value]) => key.startsWith("reusableTab:") && key !== storageKey && value?.tab_id)
    .map(([, value]) => value.tab_id));
  const tabs = await chrome.tabs.query({});
  const tab = tabs.find((value) => !leasedTabIds.has(value.id) && !ownedByOtherKeys.has(value.id) && candidates.has(comparableURL(value.url))) || null;
  if (tab) leasedTabIds.add(tab.id);
  return tab;
}

async function rememberReusableTab(reuseKey, tab, groupTitle) {
  const storageKey = reusableTabStorageKey(reuseKey);
  const record = { tab_id: tab.id, final_url: comparableURL(tab.url), group_title: groupTitle };
  await Promise.all([
    chrome.storage.session.set({ [storageKey]: { tab_id: tab.id } }),
    chrome.storage.local.set({ [storageKey]: record })
  ]);
}

async function findTabGroup(windowId, title) {
  const groups = await chrome.tabGroups.query(windowId == null ? {} : { windowId });
  return groups.find((group) => group.title === title) || null;
}

async function tabBelongsToGroup(tab, title) {
  if (!title || tab.groupId == null || tab.groupId === chrome.tabGroups.TAB_GROUP_ID_NONE) return false;
  const group = await chrome.tabGroups.get(tab.groupId).catch(() => null);
  return group?.title === title;
}

function reusableTabStorageKey(reuseKey) {
  return `reusableTab:${reuseKey}`;
}

function comparableURL(value) {
  try {
    const parsed = new URL(value);
    parsed.hash = "";
    return parsed.href;
  } catch {
    return "";
  }
}

async function runInTab(tabId, func, args) {
  const results = await chrome.scripting.executeScript({ target: { tabId }, func, args, world: "ISOLATED" });
  return results[0]?.result ?? {};
}

async function runInMainWorld(tabId, func, args) {
  const results = await chrome.scripting.executeScript({ target: { tabId }, func, args, world: "MAIN" });
  return { value: results[0]?.result ?? null };
}

function documentSnapshot(maxLength) {
  const html = document.documentElement?.outerHTML || "";
  return { url: location.href, title: document.title, html: html.slice(0, maxLength), truncated: html.length > maxLength };
}

function queryElement(selector, maxLength) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Selector not found: ${selector}`);
  const attributes = Object.fromEntries(Array.from(element.attributes || []).map((item) => [item.name, item.value]));
  const text = (element.innerText || element.textContent || "").trim();
  const html = element.outerHTML || "";
  return { url: location.href, title: document.title, selector, tag_name: element.tagName.toLowerCase(), text: text.slice(0, maxLength), html: html.slice(0, maxLength), attributes, truncated: text.length > maxLength || html.length > maxLength };
}

function clickElement(selector) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Selector not found: ${selector}`);
  element.scrollIntoView({ block: "center", inline: "center" });
  element.click();
  return { selector, clicked: true };
}

function inputElement(selector, value) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Selector not found: ${selector}`);
  element.focus();
  if (element instanceof HTMLInputElement) setNativeControlValue(element, value, HTMLInputElement.prototype);
  else if (element instanceof HTMLTextAreaElement) setNativeControlValue(element, value, HTMLTextAreaElement.prototype);
  else if (element instanceof HTMLSelectElement) setNativeControlValue(element, value, HTMLSelectElement.prototype);
  else if (element instanceof HTMLElement && element.isContentEditable) element.textContent = value;
  else throw new Error(`Selector is not an input control: ${selector}`);
  element.dispatchEvent(new Event("input", { bubbles: true }));
  element.dispatchEvent(new Event("change", { bubbles: true }));
  return { selector, updated: true };
}

function setNativeControlValue(element, value, prototype) {
  const setter = Object.getOwnPropertyDescriptor(prototype, "value")?.set;
  if (!setter) throw new Error("The input control does not expose a native value setter.");
  setter.call(element, value);
}

function evaluateExpression(expression) {
  const value = (0, eval)(expression);
  if (value == null || ["string", "number", "boolean"].includes(typeof value)) return value;
  return JSON.parse(JSON.stringify(value));
}

async function waitForTab(tabId, deadline) {
  const current = await chrome.tabs.get(tabId);
  if (current.status === "complete") return;
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => { cleanup(); reject(new Error("Page load timed out.")); }, remaining(deadline));
    const listener = (updatedTabId, changeInfo) => {
      if (updatedTabId === tabId && changeInfo.status === "complete") { cleanup(); resolve(); }
    };
    const removed = (removedTabId) => { if (removedTabId === tabId) { cleanup(); reject(new Error("Tab was closed.")); } };
    const cleanup = () => { clearTimeout(timer); chrome.tabs.onUpdated.removeListener(listener); chrome.tabs.onRemoved.removeListener(removed); };
    chrome.tabs.onUpdated.addListener(listener);
    chrome.tabs.onRemoved.addListener(removed);
  });
}

async function waitForSelector(tabId, selector, timeoutMS) {
  const interval = 150;
  const started = Date.now();
  while (Date.now() - started < timeoutMS) {
    const result = await runInTab(tabId, (value) => Boolean(document.querySelector(value)), [selector]);
    if (result) return;
    await sleep(interval);
  }
  throw new Error(`Selector wait timed out: ${selector}`);
}

async function createNetworkCapture(tabId, rules) {
  const target = { tabId };
  const results = [];
  const requests = new Map();
  const responses = new Map();
  const pending = new Set();
  let sequence = 0;
  await chrome.debugger.attach(target, "1.3");
  await chrome.debugger.sendCommand(target, "Network.enable", { maxTotalBufferSize: 50 * 1024 * 1024, maxResourceBufferSize: 10 * 1024 * 1024 });
  const listener = (source, method, params) => {
    if (source.tabId !== tabId) return;
    if (method === "Network.requestWillBeSent") requests.set(params.requestId, {
      method: params.request?.method || "",
      headers: normalizeHeaders(params.request?.headers || {}),
      postData: String(params.request?.postData || ""),
      timestamp: Number(params.timestamp) || 0,
      initiatorType: String(params.initiator?.type || "")
    });
    if (method === "Network.responseReceived") {
      for (const rule of rules) {
        const response = params.response || {};
        if ((!rule.url_contains || response.url.includes(rule.url_contains)) && (!rule.resource_type || String(params.type).toLowerCase() === String(rule.resource_type).toLowerCase())) {
          responses.set(params.requestId, { rule, response, resourceType: params.type, sequence: sequence++ });
        }
      }
    }
    if (method === "Network.loadingFinished" && responses.has(params.requestId)) {
      const task = collectResponse(target, params.requestId, responses.get(params.requestId), requests.get(params.requestId), params, results).finally(() => pending.delete(task));
      pending.add(task);
    }
    if (method === "Network.loadingFailed" && responses.has(params.requestId)) {
      const matched = responses.get(params.requestId);
      const request = requests.get(params.requestId);
      results.push({ capture_id: matched.rule.id, url: matched.response.url, method: request?.method || "", status: matched.response.status || 0, resource_type: matched.resourceType, request_headers: request?.headers || {}, request_body: request?.postData || "", initiator_type: request?.initiatorType || "", duration_ms: request?.timestamp && params.timestamp ? Math.max(0, Math.round((params.timestamp - request.timestamp) * 1000)) : 0, error: params.errorText || "Network request failed.", _sequence: matched.sequence });
    }
  };
  chrome.debugger.onEvent.addListener(listener);
  let stopped = false;
  return {
    results,
    async flush(deadline) {
      while (pending.size && remaining(deadline) > 0) await Promise.race([Promise.allSettled([...pending]), sleep(Math.min(100, remaining(deadline)))]);
      results.sort((left, right) => left._sequence - right._sequence);
      for (const result of results) delete result._sequence;
    },
    async stop() {
      if (stopped) return;
      stopped = true;
      chrome.debugger.onEvent.removeListener(listener);
      await chrome.debugger.detach(target).catch(() => {});
    }
  };
}

async function collectResponse(target, requestId, matched, request, loading, results) {
  const response = matched.response;
  const maxBytes = clamp(Number(matched.rule.max_body_bytes) || 262144, 1024, 1048576);
  const output = {
    capture_id: matched.rule.id,
    url: response.url,
    method: request?.method || "",
    status: Math.round(response.status || 0),
    status_text: response.statusText || "",
    resource_type: matched.resourceType || "",
    mime_type: response.mimeType || "",
    protocol: response.protocol || "",
    remote_ip_address: response.remoteIPAddress || "",
    remote_port: Math.round(response.remotePort || 0),
    initiator_type: request?.initiatorType || "",
    headers: normalizeHeaders(response.headers || {}),
    request_headers: request?.headers || {},
    request_body: String(request?.postData || "").slice(0, maxBytes),
    request_body_truncated: String(request?.postData || "").length > maxBytes,
    encoded_data_length: Math.round(loading?.encodedDataLength || response.encodedDataLength || 0),
    duration_ms: request?.timestamp && loading?.timestamp ? Math.max(0, Math.round((loading.timestamp - request.timestamp) * 1000)) : 0,
    from_disk_cache: Boolean(response.fromDiskCache),
    from_service_worker: Boolean(response.fromServiceWorker),
    timing: normalizeTiming(response.timing || {}),
    _sequence: matched.sequence
  };
  try {
    const body = await chrome.debugger.sendCommand(target, "Network.getResponseBody", { requestId });
    output.body = String(body.body || "").slice(0, maxBytes);
    output.body_base64 = Boolean(body.base64Encoded);
    output.truncated = String(body.body || "").length > maxBytes;
  } catch (error) {
    output.error = safeError(error);
  }
  results.push(output);
}

function normalizeHeaders(headers) {
  return Object.fromEntries(Object.entries(headers).map(([key, value]) => [String(key), String(value)]));
}

function normalizeTiming(timing) {
  return Object.fromEntries(Object.entries(timing).filter(([, value]) => Number.isFinite(value)).map(([key, value]) => [String(key), Number(value)]));
}

function tabInfo(tab) {
  return { tab_id: tab.id, window_id: tab.windowId, url: tab.url || "", title: tab.title || "", status: tab.status || "" };
}
function requireTab(state) { if (!state.tabId) throw new Error("No browser tab is active for this run."); }
function requiredSelector(value) { const selector = String(value || ""); if (!selector || selector.length > 4096) throw new Error("A selector between 1 and 4096 characters is required."); return selector; }
function validateURL(value, allowAbout = false) { const url = new URL(value); if (!(url.protocol === "http:" || url.protocol === "https:" || (allowAbout && url.href === "about:blank"))) throw new Error("Only HTTP and HTTPS URLs are supported."); }
function validGroupColor(value) { return ["grey", "blue", "red", "yellow", "green", "pink", "purple", "cyan", "orange"].includes(value) ? value : "blue"; }
function clamp(value, minimum, maximum) { return Math.min(maximum, Math.max(minimum, Number.isFinite(value) ? value : minimum)); }
function remaining(deadline) { return Math.max(1, deadline - Date.now()); }
function sleep(milliseconds) { return new Promise((resolve) => setTimeout(resolve, milliseconds)); }
async function withTimeout(promise, milliseconds, message) {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(message)), milliseconds); })]);
  } finally {
    clearTimeout(timer);
  }
}
function safeError(error) { return String(error?.message || error || "Browser operation failed.").slice(0, 2000); }

void ensureIdentity().then(connect);
