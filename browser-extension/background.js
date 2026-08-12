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
  "runtime.evaluate", "network.start", "network.stop", "browser.targets"
];
const RESPONSE_CHUNK_SIZE = 512 * 1024;
const MAX_RESPONSE_SIZE = 60 * 1024 * 1024;
const MAX_SOCKET_BUFFER = 4 * 1024 * 1024;

let socket = null;
let reconnectTimer = null;
let heartbeatTimer = null;
let reconnectAttempt = 0;
let connectionState = "disconnected";
let lastError = "";
let activeRuns = 0;
const networkSessions = new Map();
let targetsChangedTimer = null;

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

for (const event of [chrome.tabs.onCreated, chrome.tabs.onUpdated, chrome.tabs.onMoved, chrome.tabs.onAttached, chrome.tabs.onDetached, chrome.tabGroups.onCreated, chrome.tabGroups.onUpdated, chrome.tabGroups.onRemoved, chrome.windows.onCreated, chrome.windows.onRemoved, chrome.windows.onFocusChanged]) {
  event?.addListener(() => scheduleTargetsChanged());
}
chrome.tabs.onRemoved.addListener((tabId) => {
  void stopSessionsForTab(tabId);
  scheduleTargetsChanged();
});

function scheduleTargetsChanged() {
  clearTimeout(targetsChangedTimer);
  targetsChangedTimer = setTimeout(() => send({ protocol: PROTOCOL_VERSION, type: "event", command: "browser.targets.changed", payload: {} }), 150);
}

async function stopSessionsForTab(tabId) {
  for (const [sessionId, entry] of networkSessions) {
    if (entry.session.target.tab_id !== tabId) continue;
    await entry.capture.stop();
    networkSessions.delete(sessionId);
    send({ protocol: PROTOCOL_VERSION, type: "event", command: "browser.network.status", payload: { ...entry.session, status: "stopped", error: "Target tab was closed." } });
  }
}

async function stopAllNetworkSessions(reason) {
  const entries = [...networkSessions.entries()];
  networkSessions.clear();
  await Promise.allSettled(entries.map(async ([sessionId, entry]) => {
    await entry.capture.stop();
    send({ protocol: PROTOCOL_VERSION, type: "event", command: "browser.network.status", payload: { ...entry.session, id: sessionId, status: "stopped", error: reason } });
  }));
}

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
    void stopAllNetworkSessions("Meerkit connection closed.");
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
  if (message.type !== "command" || !message.id) return;
  const config = await settings();
  if (activeRuns >= Math.max(1, Number(config.maxConcurrent) || 1)) {
    send({ protocol: PROTOCOL_VERSION, type: "response", id: message.id, error: "browser agent concurrency limit reached" });
    return;
  }
  activeRuns++;
  try {
    const result = await executeCommand(message.command, message.payload || {});
    await sendCommandResult(message.id, result);
  } catch (error) {
    send({ protocol: PROTOCOL_VERSION, type: "response", id: message.id, error: safeError(error) });
  } finally {
    activeRuns--;
  }
}

async function sendCommandResult(id, result) {
  const serialized = JSON.stringify(result);
  if (serialized.length <= RESPONSE_CHUNK_SIZE) {
    send({ protocol: PROTOCOL_VERSION, type: "response", id, payload: result });
    return;
  }
  if (serialized.length > MAX_RESPONSE_SIZE) throw new Error("Browser result exceeds 60 MiB. Use WebP or JPEG for a smaller full-page screenshot.");
  const boundaries = [0];
  const sendDeadline = Date.now() + 60000;
  for (let offset = 0; offset < serialized.length;) {
    let end = Math.min(serialized.length, offset + RESPONSE_CHUNK_SIZE);
    if (end < serialized.length && isHighSurrogate(serialized.charCodeAt(end - 1))) end--;
    boundaries.push(end);
    offset = end;
  }
  const total = boundaries.length - 1;
  for (let sequence = 0; sequence < total; sequence++) {
    while (socket?.readyState === WebSocket.OPEN && socket.bufferedAmount > MAX_SOCKET_BUFFER) {
      if (Date.now() >= sendDeadline) throw new Error("Timed out while sending the browser result to Meerkit.");
      await sleep(10);
    }
    if (!socket || socket.readyState !== WebSocket.OPEN) throw new Error("Meerkit connection closed while sending the browser result.");
    send({ protocol: PROTOCOL_VERSION, type: "response_chunk", id, sequence, total, chunk: serialized.slice(boundaries[sequence], boundaries[sequence + 1]) });
  }
}

function isHighSurrogate(value) { return value >= 0xD800 && value <= 0xDBFF; }

async function executeCommand(command, request) {
  if (command === "browser.targets") return listTargets();
  if (command === "browser.action") return executeSingleAction(request);
  if (command === "browser.network.start") return startNetworkSession(request);
  if (command === "browser.network.stop") return stopNetworkSession(request);
  throw new Error(`Unsupported browser command: ${command}`);
}

async function executeSingleAction(request) {
  const timeoutMS = clamp(Number(request.timeout_ms) || 60000, 1000, 300000);
  const deadline = Date.now() + timeoutMS;
  const action = request.action || {};
  const state = { tabId: positiveInteger(request.target?.tab_id), windowId: positiveInteger(request.target?.window_id), createdTab: false, debuggerAttached: false, capture: null, pendingCaptureRules: [], networkResults: [] };
  if (state.tabId) { const tab = await chrome.tabs.get(state.tabId); if (state.windowId && tab.windowId !== state.windowId) throw new Error("Selected tab does not belong to the selected window."); state.windowId = tab.windowId; }
  const actionStarted = performance.now();
  const data = await withTimeout(executeAction(state, action, [], deadline), remaining(deadline), `Action ${action.id || action.type} timed out.`);
  const target = state.tabId ? { agent_id: await ensureIdentity(), window_id: state.windowId, tab_id: state.tabId } : {};
  return { id: action.id, type: action.type, success: true, duration_ms: Math.round(performance.now() - actionStarted), target, data: data || {} };
}

async function listTargets() {
  const windows = await chrome.windows.getAll({ populate: true });
  const groups = await chrome.tabGroups.query({}).catch(() => []);
  const groupsById = new Map(groups.map((group) => [group.id, group]));
  return { agent_id: await ensureIdentity(), windows: windows.map((window) => ({ id: window.id, focused: Boolean(window.focused), type: window.type || "normal", tabs: (window.tabs || []).map((tab) => ({ id: tab.id, window_id: tab.windowId, index: tab.index, active: Boolean(tab.active), status: tab.status || "", title: tab.title || "", url: tab.url || "", group_id: tab.groupId, group_title: groupsById.get(tab.groupId)?.title || "" })) })) };
}

async function startNetworkSession(request) {
  const sessionId = String(request.session_id || crypto.randomUUID());
  const tabId = positiveInteger(request.target?.tab_id);
  if (!tabId) throw new Error("Network capture requires tab_id.");
  const tab = await chrome.tabs.get(tabId);
  if (request.target?.window_id && tab.windowId !== request.target.window_id) throw new Error("Selected tab does not belong to the selected window.");
  if (networkSessions.has(sessionId)) throw new Error("Network capture session already exists.");
  const capture = await createNetworkCapture(tabId, request.rules || [], sessionId);
  const session = { id: sessionId, target: { agent_id: await ensureIdentity(), window_id: tab.windowId, tab_id: tabId }, status: "running", started_at: new Date().toISOString(), count: 0 };
  networkSessions.set(sessionId, { session, capture });
  return session;
}

async function stopNetworkSession(request) {
  const sessionId = String(request.session_id || request.id || "");
  const entry = networkSessions.get(sessionId);
  if (!entry) throw new Error("Network capture session was not found.");
  await entry.capture.flush(Date.now() + 10000);
  await entry.capture.stop();
  networkSessions.delete(sessionId);
  return { ...entry.session, status: "stopped", count: entry.capture.results.length, events: entry.capture.results };
}

async function executeAction(state, action, _captureRules, deadline) {
  const params = action.params || {};
  switch (action.type) {
    case "tab.open": {
	  if (state.tabId) throw new Error("tab.open does not accept an existing tab target.");
      validateURL(params.url || "about:blank", true);
	  let tab = await chrome.tabs.create({ url: "about:blank", active: params.active !== false, ...(state.windowId ? { windowId: state.windowId } : {}) });
      state.tabId = tab.id;
      state.windowId = tab.windowId;
	  if (params.url && params.url !== "about:blank") {
        await chrome.tabs.update(tab.id, { url: params.url });
        if (params.wait !== false) await waitForTab(tab.id, deadline);
      }
      const finalTab = await chrome.tabs.get(tab.id);
	  return tabInfo(finalTab);
    }
    case "tab.navigate": {
      requireTab(state);
      validateURL(params.url);
      await chrome.tabs.update(state.tabId, { url: params.url });
      if (params.wait !== false) await waitForTab(state.tabId, deadline);
      const tab = await chrome.tabs.get(state.tabId);
      state.windowId = tab.windowId;
      return tabInfo(tab);
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
      await chrome.tabs.remove(tabId);
      state.tabId = null;
	  state.createdTab = false;
      state.capture = null;
      state.debuggerAttached = false;
      return { tab_id: tabId };
    }
    case "page.wait": {
      requireTab(state);
      const mode = params.mode || (params.selector ? "selector" : params.duration_ms != null ? "duration" : "load");
      if (mode === "selector") await waitForSelector(state.tabId, String(params.selector || ""), clamp(Number(params.timeout_ms) || remaining(deadline), 100, remaining(deadline)));
      else if (mode === "duration") await sleep(clamp(Number(params.duration_ms), 0, remaining(deadline)));
      else await waitForTab(state.tabId, deadline);
      return { ready: true, mode };
    }
    case "page.screenshot": {
      requireTab(state);
      const format = ["jpeg", "webp"].includes(params.format) ? params.format : "png";
      const target = { tabId: state.tabId };
      let attachedHere = false;
      if (!state.debuggerAttached) {
        await chrome.debugger.attach(target, "1.3");
        attachedHere = true;
      }
      try {
        const screenshot = await chrome.debugger.sendCommand(target, "Page.captureScreenshot", {
          format,
          ...(format !== "png" ? { quality: clamp(Number(params.quality) || 90, 1, 100) } : {}),
          captureBeyondViewport: Boolean(params.full_page)
        });
        return { data_url: `data:image/${format};base64,${screenshot.data}`, format, full_page: Boolean(params.full_page), size_bytes: Math.floor(screenshot.data.length * 3 / 4) };
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

async function findTabGroup(windowId, title) {
  const groups = await chrome.tabGroups.query(windowId == null ? {} : { windowId });
  return groups.find((group) => group.title === title) || null;
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

async function createNetworkCapture(tabId, rules, sessionId) {
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
      const task = collectResponse(target, params.requestId, responses.get(params.requestId), requests.get(params.requestId), params, results).then((result) => {
        if (result) {
          result.session_id = sessionId;
          send({ protocol: PROTOCOL_VERSION, type: "event", command: "browser.network", payload: result });
        }
      }).finally(() => pending.delete(task));
      pending.add(task);
    }
    if (method === "Network.loadingFailed" && responses.has(params.requestId)) {
      const matched = responses.get(params.requestId);
      const request = requests.get(params.requestId);
      const result = { session_id: sessionId, capture_id: matched.rule.id, url: matched.response.url, method: request?.method || "", status: matched.response.status || 0, resource_type: matched.resourceType, request_headers: request?.headers || {}, request_body: request?.postData || "", initiator_type: request?.initiatorType || "", duration_ms: request?.timestamp && params.timestamp ? Math.max(0, Math.round((params.timestamp - request.timestamp) * 1000)) : 0, error: params.errorText || "Network request failed.", _sequence: matched.sequence };
      results.push(result);
      send({ protocol: PROTOCOL_VERSION, type: "event", command: "browser.network", payload: result });
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
  return output;
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
function requireTab(state) { if (!state.tabId) throw new Error("This browser action requires tab_id."); }
function requiredSelector(value) { const selector = String(value || ""); if (!selector || selector.length > 4096) throw new Error("A selector between 1 and 4096 characters is required."); return selector; }
function validateURL(value, allowAbout = false) { const url = new URL(value); if (!(url.protocol === "http:" || url.protocol === "https:" || (allowAbout && url.href === "about:blank"))) throw new Error("Only HTTP and HTTPS URLs are supported."); }
function validGroupColor(value) { return ["grey", "blue", "red", "yellow", "green", "pink", "purple", "cyan", "orange"].includes(value) ? value : "blue"; }
function clamp(value, minimum, maximum) { return Math.min(maximum, Math.max(minimum, Number.isFinite(value) ? value : minimum)); }
function positiveInteger(value) { const number = Math.trunc(Number(value) || 0); return number > 0 ? number : null; }
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
