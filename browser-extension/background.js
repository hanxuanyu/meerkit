importScripts("modules/config.js", "modules/action-badge.js", "modules/debug-controller.js");

const PROTOCOL_VERSION = MeerkitConfig.protocolVersion;
const EXTENSION_VERSION = chrome.runtime.getManifest().version;
const DEFAULT_SETTINGS = MeerkitConfig.defaultSettings;
const CAPABILITIES = MeerkitConfig.capabilities;
const RESPONSE_CHUNK_SIZE = MeerkitConfig.responseChunkSize;
const MAX_RESPONSE_SIZE = MeerkitConfig.maxResponseSize;
const MAX_SOCKET_BUFFER = MeerkitConfig.maxSocketBuffer;

let socket = null;
let reconnectTimer = null;
let heartbeatTimer = null;
let reconnectAttempt = 0;
let connectionState = "disconnected";
let lastError = "";
let activeRuns = 0;
const networkSessions = new Map();
const debuggerSessions = new Map();
const inputQueues = new Map();
let targetsChangedTimer = null;
const actionBadge = MeerkitActionBadge.create(chrome);
const debugController = MeerkitDebugController.create(chrome);
actionBadge.update(connectionState, activeRuns);
void debugController.initialize();

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
  if (["debug.status", "debug.set", "debug.inspector.disabled"].includes(message?.type)) {
    void debugController.handleMessage(message, _sender).then(sendResponse).catch((error) => sendResponse({ error: safeError(error) }));
    return true;
  }
  return false;
});

for (const event of [chrome.tabs.onCreated, chrome.tabs.onUpdated, chrome.tabs.onMoved, chrome.tabs.onActivated, chrome.tabs.onAttached, chrome.tabs.onDetached, chrome.tabGroups.onCreated, chrome.tabGroups.onUpdated, chrome.tabGroups.onRemoved, chrome.windows.onCreated, chrome.windows.onRemoved, chrome.windows.onFocusChanged, chrome.windows.onBoundsChanged]) {
  event?.addListener(() => scheduleTargetsChanged());
}
chrome.tabs.onRemoved.addListener((tabId) => {
  void stopSessionsForTab(tabId);
  debuggerSessions.delete(tabId);
  inputQueues.delete(tabId);
  scheduleTargetsChanged();
});
chrome.debugger.onDetach?.addListener((source, reason) => {
  if (source.tabId) void handleDebuggerDetach(source.tabId, reason);
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

async function handleDebuggerDetach(tabId, reason) {
  debuggerSessions.delete(tabId);
  for (const [sessionId, entry] of networkSessions) {
    if (entry.session.target.tab_id !== tabId) continue;
    await entry.capture.stop();
    networkSessions.delete(sessionId);
    send({ protocol: PROTOCOL_VERSION, type: "event", command: "browser.network.status", payload: { ...entry.session, status: "stopped", error: `Chrome debugger detached: ${reason || "unknown reason"}.` } });
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
  actionBadge.update(connectionState, activeRuns);
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
  actionBadge.update(connectionState, activeRuns);
  try {
    const result = await executeCommand(message.command, message.payload || {});
    await sendCommandResult(message.id, result);
  } catch (error) {
    send({ protocol: PROTOCOL_VERSION, type: "response", id: message.id, error: safeError(error) });
  } finally {
    activeRuns = Math.max(0, activeRuns - 1);
    actionBadge.update(connectionState, activeRuns);
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
  if (command === "browser.selector_candidates") return selectorCandidates(request);
  if (command === "browser.action") return executeSingleAction(request);
  if (command === "browser.network.start") return startNetworkSession(request);
  if (command === "browser.network.stop") return stopNetworkSession(request);
  throw new Error(`Unsupported browser command: ${command}`);
}

async function selectorCandidates(request) {
  const tabId = positiveInteger(request.target?.tab_id);
  if (!tabId) throw new Error("Selector candidates require tab_id.");
  const tab = await chrome.tabs.get(tabId);
  if (request.target?.window_id && tab.windowId !== request.target.window_id) throw new Error("Selected tab does not belong to the selected window.");
  const queries = Array.isArray(request.queries) ? request.queries.map((value) => String(value).trim()).filter(Boolean) : [];
  if (!queries.length || queries.length > 16 || queries.some((query) => query.length > 4096)) throw new Error("Between 1 and 16 valid selector candidate queries are required.");
  const limit = clamp(Number(request.limit) || 50, 1, 200);
  return runInTab(tabId, collectSelectorCandidates, [queries, limit]);
}

async function executeSingleAction(request) {
  const timeoutMS = clamp(Number(request.timeout_ms) || 60000, 1000, 300000);
  const deadline = Date.now() + timeoutMS;
  const action = request.action || {};
  const state = { tabId: positiveInteger(request.target?.tab_id), windowId: positiveInteger(request.target?.window_id) };
  if (state.tabId) { const tab = await chrome.tabs.get(state.tabId); if (state.windowId && tab.windowId !== state.windowId) throw new Error("Selected tab does not belong to the selected window."); state.windowId = tab.windowId; }
  const actionStarted = performance.now();
  const data = await withTimeout(executeAction(state, action, [], deadline), remaining(deadline), `Action ${action.id || action.type} timed out.`);
  const target = state.tabId ? { agent_id: await ensureIdentity(), window_id: state.windowId, tab_id: state.tabId } : state.windowId ? { agent_id: await ensureIdentity(), window_id: state.windowId } : {};
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
  const capture = await createNetworkCapture(tabId, request.rules || [], sessionId, Boolean(request.disable_cache));
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
    case "window.open": {
      validateURL(params.url || "about:blank", true);
      const stateValue = validWindowState(params.state);
      const options = { url: params.url || "about:blank", type: params.type === "popup" ? "popup" : "normal", state: stateValue };
      if (stateValue === "normal") for (const key of ["width", "height", "left", "top"]) if (params[key] != null && params[key] !== "") options[key] = Math.trunc(Number(params[key]));
      let window = await chrome.windows.create(options);
      if (!window.tabs?.length) window = await chrome.windows.get(window.id, { populate: true });
      state.windowId = window.id;
      state.tabId = window.tabs?.[0]?.id || null;
      return windowInfo(window);
    }
    case "window.focus":
      requireWindow(state);
      return windowInfo(await chrome.windows.update(state.windowId, { focused: true }));
    case "window.state":
      requireWindow(state);
      return windowInfo(await chrome.windows.update(state.windowId, { state: validWindowState(params.state) }));
    case "window.resize": {
      requireWindow(state);
      const update = { state: "normal", width: clamp(Math.trunc(Number(params.width)), 100, 10000), height: clamp(Math.trunc(Number(params.height)), 100, 10000) };
      for (const key of ["left", "top"]) if (params[key] != null && params[key] !== "") update[key] = clamp(Math.trunc(Number(params[key])), -10000, 10000);
      return windowInfo(await chrome.windows.update(state.windowId, update));
    }
    case "window.close": {
      requireWindow(state);
      const windowId = state.windowId;
      await chrome.windows.remove(windowId);
      state.windowId = null;
      state.tabId = null;
      return { window_id: windowId, closed: true };
    }
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
    case "tab.activate": {
      requireTab(state);
      const tab = await chrome.tabs.update(state.tabId, { active: true });
      await chrome.windows.update(tab.windowId, { focused: true });
      state.windowId = tab.windowId;
      return tabInfo(tab);
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
    case "tab.reload":
      requireTab(state);
      await chrome.tabs.reload(state.tabId, { bypassCache: Boolean(params.bypass_cache) });
      if (params.wait !== false) await waitForTab(state.tabId, deadline);
      return tabInfo(await chrome.tabs.get(state.tabId));
    case "tab.back":
      requireTab(state);
      await chrome.tabs.goBack(state.tabId);
      return tabInfo(await chrome.tabs.get(state.tabId));
    case "tab.forward":
      requireTab(state);
      await chrome.tabs.goForward(state.tabId);
      return tabInfo(await chrome.tabs.get(state.tabId));
    case "tab.duplicate": {
      requireTab(state);
      const tab = await chrome.tabs.duplicate(state.tabId);
      if (params.active !== false) await chrome.tabs.update(tab.id, { active: true });
      state.tabId = tab.id;
      state.windowId = tab.windowId;
      return tabInfo(await chrome.tabs.get(tab.id));
    }
    case "tab.move": {
      requireTab(state);
      const move = { index: Math.trunc(Number(params.index ?? -1)) };
      if (positiveInteger(params.destination_window_id)) move.windowId = positiveInteger(params.destination_window_id);
      const tab = await chrome.tabs.move(state.tabId, move);
      state.windowId = tab.windowId;
      return tabInfo(tab);
    }
    case "tab.pin":
      requireTab(state);
      return tabInfo(await chrome.tabs.update(state.tabId, { pinned: params.pinned !== false }));
    case "tab.mute":
      requireTab(state);
      return tabInfo(await chrome.tabs.update(state.tabId, { muted: params.muted !== false }));
    case "tab.discard": {
      requireTab(state);
      const tab = await chrome.tabs.discard(state.tabId);
      if (!tab) throw new Error("Chrome did not discard the tab.");
      return tabInfo(tab);
    }
    case "tab.auto_discardable":
      requireTab(state);
      return tabInfo(await chrome.tabs.update(state.tabId, { autoDiscardable: params.auto_discardable !== false }));
    case "tab.detect_language":
      requireTab(state);
      return { tab_id: state.tabId, language: await chrome.tabs.detectLanguage(state.tabId) };
    case "tab.group": {
      requireTab(state);
      const title = String(params.title || "Meerkit");
      const current = await chrome.tabs.get(state.tabId);
      if (current.groupId != null && current.groupId !== chrome.tabGroups.TAB_GROUP_ID_NONE) {
        const currentGroup = await chrome.tabGroups.get(current.groupId).catch(() => null);
        if (currentGroup?.title === title) {
          const group = await chrome.tabGroups.update(currentGroup.id, { title, color: validGroupColor(params.color), collapsed: Boolean(params.collapsed) });
          return { ...groupInfo(group || currentGroup), reused: true };
        }
      }
      const existing = params.reuse_group !== false ? await findTabGroup(current.windowId, title) : null;
      const groupId = await chrome.tabs.group({ tabIds: [state.tabId], ...(existing ? { groupId: existing.id } : {}) });
      const group = await chrome.tabGroups.update(groupId, { title, color: validGroupColor(params.color), collapsed: Boolean(params.collapsed) });
      return { ...groupInfo(group || { id: groupId, title, color: validGroupColor(params.color), collapsed: Boolean(params.collapsed), windowId: current.windowId }), reused: Boolean(existing) };
    }
    case "tab.ungroup":
      requireTab(state);
      await chrome.tabs.ungroup([state.tabId]);
      return tabInfo(await chrome.tabs.get(state.tabId));
    case "tab.zoom":
      requireTab(state);
      await chrome.tabs.setZoom(state.tabId, clamp(Number(params.factor), 0.25, 5));
      return { tab_id: state.tabId, factor: await chrome.tabs.getZoom(state.tabId) };
    case "tab.close": {
      requireTab(state);
      const tabId = state.tabId;
      await chrome.tabs.remove(tabId);
      state.tabId = null;
      return { tab_id: tabId };
    }
    case "page.info":
      requireTab(state);
      return runInTab(state.tabId, pageInformation, []);
    case "page.wait": {
      requireTab(state);
      const mode = params.mode || (params.selector ? "selector" : params.duration_ms != null ? "duration" : "load");
      if (["selector", "visible", "hidden"].includes(mode)) await waitForPageCondition(state.tabId, mode, requiredSelector(params.selector), clamp(Number(params.timeout_ms) || remaining(deadline), 100, remaining(deadline)));
      else if (["text", "url", "title"].includes(mode)) await waitForPageCondition(state.tabId, mode, String(params.value || ""), clamp(Number(params.timeout_ms) || remaining(deadline), 100, remaining(deadline)));
      else if (mode === "duration") await sleep(clamp(Number(params.duration_ms), 0, remaining(deadline)));
      else await waitForTab(state.tabId, deadline);
      return { ready: true, mode };
    }
    case "page.scroll":
      requireTab(state);
      return runInTab(state.tabId, scrollPage, [params.mode || "relative", Number(params.x) || 0, Number(params.y) || 0, params.behavior === "smooth" ? "smooth" : "auto"]);
    case "page.stop_loading":
      requireTab(state);
      return withDebugger(state.tabId, async (target) => { await chrome.debugger.sendCommand(target, "Page.stopLoading"); return { tab_id: state.tabId, stopped: true }; });
    case "page.performance":
      requireTab(state);
      return runInTab(state.tabId, performanceSnapshot, []);
    case "page.screenshot": {
      requireTab(state);
      const format = ["jpeg", "webp"].includes(params.format) ? params.format : "png";
      return withDebugger(state.tabId, async (target) => {
        const screenshot = await chrome.debugger.sendCommand(target, "Page.captureScreenshot", {
          format,
          ...(format !== "png" ? { quality: clamp(Number(params.quality) || 90, 1, 100) } : {}),
          captureBeyondViewport: Boolean(params.full_page)
        });
        return { data_url: `data:image/${format};base64,${screenshot.data}`, format, full_page: Boolean(params.full_page), size_bytes: Math.floor(screenshot.data.length * 3 / 4) };
      });
    }
    case "dom.document":
      requireTab(state);
      return runInTab(state.tabId, documentSnapshot, [clamp(Number(params.max_length) || 262144, 1024, 1048576)]);
    case "dom.query":
      requireTab(state);
      return runInTab(state.tabId, queryElement, [requiredSelector(params.selector), clamp(Number(params.max_length) || 65536, 256, 1048576)]);
    case "dom.query_all":
      requireTab(state);
      return runInTab(state.tabId, queryElements, [requiredSelector(params.selector), clamp(Number(params.limit) || 50, 1, 500), clamp(Number(params.max_length) || 4096, 64, 65536)]);
    case "dom.focus":
      requireTab(state);
      return runInTab(state.tabId, focusElement, [requiredSelector(params.selector)]);
    case "dom.blur":
      requireTab(state);
      return runInTab(state.tabId, blurElement, [requiredSelector(params.selector)]);
    case "dom.click":
      requireTab(state);
      return runInTab(state.tabId, clickElement, [requiredSelector(params.selector)]);
    case "dom.input":
      requireTab(state);
      return runInMainTab(state.tabId, inputElement, [requiredSelector(params.selector), String(params.value ?? "")]);
    case "dom.check":
      requireTab(state);
      return runInMainTab(state.tabId, checkElement, [requiredSelector(params.selector), params.checked !== false]);
    case "dom.select":
      requireTab(state);
      return runInMainTab(state.tabId, selectElement, [requiredSelector(params.selector), String(params.value ?? "")]);
    case "dom.submit":
      requireTab(state);
      return runInTab(state.tabId, submitForm, [requiredSelector(params.selector)]);
    case "dom.set_attribute":
      requireTab(state);
      return runInTab(state.tabId, setElementAttribute, [requiredSelector(params.selector), String(params.name), String(params.value ?? "")]);
    case "dom.remove_attribute":
      requireTab(state);
      return runInTab(state.tabId, removeElementAttribute, [requiredSelector(params.selector), String(params.name)]);
    case "dom.dispatch_event":
      requireTab(state);
      return runInTab(state.tabId, dispatchElementEvent, [requiredSelector(params.selector), validDOMEvent(params.event), params.bubbles !== false, params.cancelable !== false]);
    case "dom.scroll_into_view":
      requireTab(state);
      return runInTab(state.tabId, scrollElementIntoView, [requiredSelector(params.selector), params.block || "center", params.inline || "nearest", params.behavior === "smooth" ? "smooth" : "auto"]);
    case "input.click":
      requireTab(state);
      return enqueueInput(state.tabId, () => realClick(state.tabId, requiredSelector(params.selector), params.button || "left", clamp(Number(params.click_count) || 1, 1, 3)));
    case "input.hover":
      requireTab(state);
      return enqueueInput(state.tabId, () => realHover(state.tabId, requiredSelector(params.selector)));
    case "input.type":
      requireTab(state);
      return enqueueInput(state.tabId, () => realType(state.tabId, requiredSelector(params.selector), String(params.text ?? ""), Boolean(params.clear), clamp(Number(params.interval_ms) || 0, 0, 5000), deadline));
    case "input.key":
      requireTab(state);
      return enqueueInput(state.tabId, () => realKey(state.tabId, params, deadline));
    case "input.wheel":
      requireTab(state);
      return enqueueInput(state.tabId, () => realWheel(state.tabId, String(params.selector || ""), Number(params.delta_x) || 0, Number(params.delta_y) || 0));
    case "cookie.list": {
      requireTab(state);
      const url = (await chrome.tabs.get(state.tabId)).url;
      const cookies = await chrome.cookies.getAll({ url, ...(params.name ? { name: String(params.name) } : {}) });
      return { url, count: cookies.length, cookies: cookies.map(cookieInfo) };
    }
    case "cookie.set": {
      requireTab(state);
      const url = (await chrome.tabs.get(state.tabId)).url;
      const details = { url, name: String(params.name), value: String(params.value ?? ""), path: String(params.path || "/"), secure: Boolean(params.secure), httpOnly: Boolean(params.http_only) };
      const sameSite = validSameSite(params.same_site);
      if (sameSite) details.sameSite = sameSite;
      if (params.domain) details.domain = String(params.domain);
      if (params.expiration_date != null && params.expiration_date !== "") details.expirationDate = Number(params.expiration_date);
      const cookie = await chrome.cookies.set(details);
      if (!cookie) throw new Error("Chrome did not create the cookie.");
      return cookieInfo(cookie);
    }
    case "cookie.delete": {
      requireTab(state);
      const url = (await chrome.tabs.get(state.tabId)).url;
      const removed = await chrome.cookies.remove({ url, name: String(params.name), ...(params.store_id ? { storeId: String(params.store_id) } : {}) });
      return { removed: Boolean(removed), name: String(params.name), url };
    }
    case "cookie.clear": {
      requireTab(state);
      const url = (await chrome.tabs.get(state.tabId)).url;
      const cookies = await chrome.cookies.getAll({ url });
      const removed = await Promise.all(cookies.map((cookie) => chrome.cookies.remove({ url: cookieURL(cookie), name: cookie.name, storeId: cookie.storeId })));
      return { url, removed_count: removed.filter(Boolean).length };
    }
    case "storage.get":
      requireTab(state);
      return runInTab(state.tabId, getWebStorage, [validStorageArea(params.area), String(params.key || ""), clamp(Number(params.max_value_length) || 65536, 1, 1048576), 4 * 1024 * 1024]);
    case "storage.set":
      requireTab(state);
      return runInTab(state.tabId, setWebStorage, [validStorageArea(params.area), String(params.key), String(params.value ?? "")]);
    case "storage.remove":
      requireTab(state);
      return runInTab(state.tabId, removeWebStorage, [validStorageArea(params.area), String(params.key)]);
    case "storage.clear":
      requireTab(state);
      return runInTab(state.tabId, clearWebStorage, [validStorageArea(params.area)]);
    case "runtime.evaluate": {
      requireTab(state);
      const expression = String(params.expression || "");
      if (!expression || expression.length > 100000) throw new Error("A JavaScript expression between 1 and 100000 characters is required.");
      return { value: await runInMainTab(state.tabId, evaluateExpression, [expression]) };
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

async function runInMainTab(tabId, func, args) {
  const results = await chrome.scripting.executeScript({ target: { tabId }, func, args, world: "MAIN" });
  return results[0]?.result ?? {};
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
  const rect = element.getBoundingClientRect();
  const style = getComputedStyle(element);
  return { url: location.href, title: document.title, selector, tag_name: element.tagName.toLowerCase(), text: text.slice(0, maxLength), html: html.slice(0, maxLength), attributes, value: "value" in element ? String(element.value ?? "").slice(0, maxLength) : undefined, checked: "checked" in element ? Boolean(element.checked) : undefined, disabled: "disabled" in element ? Boolean(element.disabled) : undefined, visible: rect.width > 0 && rect.height > 0 && style.visibility !== "hidden" && style.display !== "none", bounding_rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height, top: rect.top, right: rect.right, bottom: rect.bottom, left: rect.left }, truncated: text.length > maxLength || html.length > maxLength || ("value" in element && String(element.value ?? "").length > maxLength) };
}

function queryElements(selector, limit, maxLength) {
  const matches = Array.from(document.querySelectorAll(selector));
  return { selector, total: matches.length, elements: matches.slice(0, limit).map((element, index) => {
    const text = (element.innerText || element.textContent || "").trim();
    const html = element.outerHTML || "";
    return { index, tag_name: element.tagName.toLowerCase(), text: text.slice(0, maxLength), html: html.slice(0, maxLength), attributes: Object.fromEntries(Array.from(element.attributes || []).map((item) => [item.name, item.value])), truncated: text.length > maxLength || html.length > maxLength };
  }), truncated: matches.length > limit };
}

function collectSelectorCandidates(queries, limit) {
  const escapeIdentifier = (value) => {
    if (globalThis.CSS?.escape) return globalThis.CSS.escape(String(value));
    const source = String(value);
    return Array.from(source).map((character, index) => {
      if (/\d/.test(character) && (index === 0 || (index === 1 && source[0] === "-"))) return `\\${character.codePointAt(0).toString(16)} `;
      return /[a-zA-Z0-9_-]/.test(character) ? character : `\\${character.codePointAt(0).toString(16)} `;
    }).join("");
  };
  const escapeString = (value) => String(value).replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/[\n\r\f]/g, " ");
  const isUnique = (selector, element) => {
    try {
      const selectorMatches = document.querySelectorAll(selector);
      return selectorMatches.length === 1 && selectorMatches[0] === element;
    } catch {
      return false;
    }
  };
  const buildSelector = (element) => {
    const tag = element.tagName.toLowerCase();
    if (element.id) {
      const selector = `#${escapeIdentifier(element.id)}`;
      if (isUnique(selector, element)) return selector;
    }
    for (const name of ["data-testid", "data-test", "data-qa", "name", "aria-label"]) {
      const value = element.getAttribute?.(name);
      if (!value) continue;
      const selector = `${tag}[${name}="${escapeString(value)}"]`;
      if (isUnique(selector, element)) return selector;
    }
    const classNames = Array.from(element.classList || []).filter((value) => value && value.length <= 80).slice(0, 3);
    if (classNames.length) {
      const selector = `${tag}${classNames.map((value) => `.${escapeIdentifier(value)}`).join("")}`;
      if (isUnique(selector, element)) return selector;
    }
    const path = [];
    let current = element;
    while (current && current.nodeType === 1 && path.length < 64) {
      const currentTag = current.tagName.toLowerCase();
      if (current.id) {
        path.unshift(`#${escapeIdentifier(current.id)}`);
      } else {
        let segment = currentTag;
        const siblings = current.parentElement ? Array.from(current.parentElement.children).filter((item) => item.tagName === current.tagName) : [];
        if (siblings.length > 1) segment += `:nth-of-type(${siblings.indexOf(current) + 1})`;
        path.unshift(segment);
      }
      const selector = path.join(" > ");
      if (isUnique(selector, element)) return selector;
      if (selector.length > 4000) break;
      current = current.parentElement;
    }
    return path.join(" > ") || tag;
  };
  const scanLimit = Math.max(limit + 1, Math.min(1000, limit * 10));
  const seen = new Set();
  const matches = [];
  let visited = 0;
  let scanTruncated = false;

  for (const query of queries) {
    let elements;
    try {
      elements = document.querySelectorAll(query);
    } catch {
      throw new Error(`Invalid selector candidate query: ${query}`);
    }
    for (const element of elements) {
      visited++;
      if (visited > 1000) {
        scanTruncated = true;
        break;
      }
      if (seen.has(element)) continue;
      seen.add(element);
      if (matches.length < limit) matches.push(element);
      if (seen.size >= scanLimit) {
        scanTruncated = true;
        break;
      }
    }
    if (scanTruncated) break;
  }

  const items = matches.map((element) => {
    const selector = buildSelector(element);
    const text = String(element.innerText || element.textContent || "").replace(/\s+/g, " ").trim().slice(0, 120);
    const attributes = {};
    for (const name of ["id", "name", "type", "role", "aria-label", "placeholder", "data-testid", "data-test", "data-qa"]) {
      const value = element.getAttribute?.(name);
      if (value) attributes[name] = String(value).slice(0, 160);
    }
    const rect = element.getBoundingClientRect();
    const style = getComputedStyle(element);
    return {
      selector,
      tag_name: element.tagName.toLowerCase(),
      text,
      attributes,
      visible: rect.width > 0 && rect.height > 0 && style.visibility !== "hidden" && style.display !== "none",
      unique: isUnique(selector, element)
    };
  });
  return { items, total: seen.size, truncated: scanTruncated || seen.size > limit };
}

function pageInformation() {
  const root = document.documentElement;
  const body = document.body;
  return { url: location.href, title: document.title, ready_state: document.readyState, viewport: { width: innerWidth, height: innerHeight, device_pixel_ratio: devicePixelRatio }, document: { width: Math.max(root?.scrollWidth || 0, body?.scrollWidth || 0), height: Math.max(root?.scrollHeight || 0, body?.scrollHeight || 0) }, scroll: { x: scrollX, y: scrollY } };
}

function performanceSnapshot() {
  const navigation = performance.getEntriesByType("navigation")[0];
  const paints = Object.fromEntries(performance.getEntriesByType("paint").map((entry) => [entry.name.replaceAll("-", "_"), Math.round(entry.startTime * 100) / 100]));
  const resources = performance.getEntriesByType("resource");
  const timing = navigation ? { type: navigation.type, start_time: navigation.startTime, duration: navigation.duration, dom_interactive: navigation.domInteractive, dom_content_loaded: navigation.domContentLoadedEventEnd, load_event_end: navigation.loadEventEnd, response_start: navigation.responseStart, response_end: navigation.responseEnd, transfer_size: navigation.transferSize, encoded_body_size: navigation.encodedBodySize, decoded_body_size: navigation.decodedBodySize } : {};
  return { url: location.href, time_origin: performance.timeOrigin, navigation: timing, paints, resources: { count: resources.length, transfer_size: resources.reduce((sum, entry) => sum + (entry.transferSize || 0), 0), encoded_body_size: resources.reduce((sum, entry) => sum + (entry.encodedBodySize || 0), 0), decoded_body_size: resources.reduce((sum, entry) => sum + (entry.decodedBodySize || 0), 0) } };
}

function scrollPage(mode, x, y, behavior) {
  const root = document.documentElement;
  if (mode === "top") scrollTo({ top: 0, left: scrollX, behavior });
  else if (mode === "bottom") scrollTo({ top: root?.scrollHeight || 0, left: scrollX, behavior });
  else if (mode === "absolute") scrollTo({ top: y, left: x, behavior });
  else scrollBy({ top: y, left: x, behavior });
  return { mode, behavior, requested: { x, y }, scroll: { x: scrollX, y: scrollY } };
}

function focusElement(selector) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Selector not found: ${selector}`);
  element.focus();
  return { selector, focused: document.activeElement === element };
}

function blurElement(selector) {
  const element = document.querySelector(selector);
  if (!(element instanceof HTMLElement)) throw new Error(`Selector is not a focusable element: ${selector}`);
  element.blur();
  return { selector, focused: document.activeElement === element };
}

function checkElement(selector, checked) {
  const element = document.querySelector(selector);
  if (!(element instanceof HTMLInputElement) || !["checkbox", "radio"].includes(element.type)) throw new Error(`Selector is not a checkbox or radio: ${selector}`);
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "checked")?.set;
  setter?.call(element, checked);
  element.dispatchEvent(new Event("input", { bubbles: true }));
  element.dispatchEvent(new Event("change", { bubbles: true }));
  return { selector, checked: element.checked };
}

function selectElement(selector, value) {
  const element = document.querySelector(selector);
  if (!(element instanceof HTMLSelectElement)) throw new Error(`Selector is not a select: ${selector}`);
  if (!Array.from(element.options).some((option) => option.value === value)) throw new Error(`Select option was not found: ${value}`);
  const setter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")?.set;
  if (!setter) throw new Error("The select control does not expose a native value setter.");
  setter.call(element, value);
  element.dispatchEvent(new Event("input", { bubbles: true }));
  element.dispatchEvent(new Event("change", { bubbles: true }));
  return { selector, value: element.value };
}

function submitForm(selector) {
  const element = document.querySelector(selector);
  const form = element instanceof HTMLFormElement ? element : element?.form;
  if (!(form instanceof HTMLFormElement)) throw new Error(`Selector is not a form or form control: ${selector}`);
  form.requestSubmit();
  return { selector, submitted: true };
}

function setElementAttribute(selector, name, value) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Selector not found: ${selector}`);
  element.setAttribute(name, value);
  return { selector, name, value: element.getAttribute(name) };
}

function removeElementAttribute(selector, name) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Selector not found: ${selector}`);
  const existed = element.hasAttribute(name);
  element.removeAttribute(name);
  return { selector, name, removed: existed };
}

function dispatchElementEvent(selector, eventName, bubbles, cancelable) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Selector not found: ${selector}`);
  const accepted = element.dispatchEvent(new Event(eventName, { bubbles, cancelable }));
  return { selector, event: eventName, bubbles, cancelable, default_prevented: !accepted };
}

function scrollElementIntoView(selector, block, inline, behavior) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Selector not found: ${selector}`);
  element.scrollIntoView({ block, inline, behavior });
  return { selector, block, inline, behavior };
}

function elementCenter(selector) {
  const element = document.querySelector(selector);
  if (!element) throw new Error(`Selector not found: ${selector}`);
  element.scrollIntoView({ block: "center", inline: "center" });
  const rect = element.getBoundingClientRect();
  if (rect.width <= 0 || rect.height <= 0) throw new Error(`Element has no clickable area: ${selector}`);
  return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
}

function webStorage(area) { return area === "session" ? sessionStorage : localStorage; }
function getWebStorage(area, key, maxValueLength, maxTotalLength) {
  const storage = webStorage(area);
  const keys = key ? [key] : Array.from({ length: storage.length }, (_, index) => storage.key(index)).filter(Boolean);
  const encoder = new TextEncoder();
  const values = {}; let truncated = false;
  const byteLength = (value) => encoder.encode(value).length;
  const truncate = (value, limit) => {
    if (byteLength(value) <= limit) return value;
    let low = 0; let high = value.length;
    while (low < high) {
      const middle = Math.ceil((low + high) / 2);
      if (byteLength(value.slice(0, middle)) <= limit) low = middle; else high = middle - 1;
    }
    const end = low > 0 && /[\uD800-\uDBFF]/.test(value[low - 1]) ? low - 1 : low;
    return value.slice(0, end);
  };
  for (const itemKey of keys) {
    const raw = storage.getItem(itemKey);
    if (raw == null) continue;
    let value = truncate(raw, maxValueLength);
    truncated ||= value.length < raw.length;
    values[itemKey] = value;
    if (byteLength(JSON.stringify({ area, count: Object.keys(values).length, values, truncated })) > maxTotalLength) {
      truncated = true;
      let low = 0; let high = value.length;
      while (low < high) {
        const middle = Math.ceil((low + high) / 2);
        values[itemKey] = value.slice(0, middle);
        const serialized = JSON.stringify({ area, count: Object.keys(values).length, values, truncated });
        if (byteLength(serialized) <= maxTotalLength) low = middle; else high = middle - 1;
      }
      const end = low > 0 && /[\uD800-\uDBFF]/.test(value[low - 1]) ? low - 1 : low;
      values[itemKey] = value.slice(0, end);
      if (!values[itemKey] && byteLength(JSON.stringify({ area, count: Object.keys(values).length, values, truncated })) > maxTotalLength) delete values[itemKey];
      break;
    }
  }
  return { area, count: Object.keys(values).length, values, truncated };
}
function setWebStorage(area, key, value) { webStorage(area).setItem(key, value); return { area, key, written: true, size: value.length }; }
function removeWebStorage(area, key) { const storage = webStorage(area); const existed = storage.getItem(key) != null; storage.removeItem(key); return { area, key, removed: existed }; }
function clearWebStorage(area) { const storage = webStorage(area); const count = storage.length; storage.clear(); return { area, removed_count: count }; }

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
  const prototype = element instanceof HTMLInputElement ? HTMLInputElement.prototype
    : element instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype
      : element instanceof HTMLSelectElement ? HTMLSelectElement.prototype
        : null;
  if (prototype) {
    const setter = Object.getOwnPropertyDescriptor(prototype, "value")?.set;
    if (!setter) throw new Error("The input control does not expose a native value setter.");
    setter.call(element, value);
  }
  else if (element instanceof HTMLElement && element.isContentEditable) element.textContent = value;
  else throw new Error(`Selector is not an input control: ${selector}`);
  element.dispatchEvent(new Event("input", { bubbles: true }));
  element.dispatchEvent(new Event("change", { bubbles: true }));
  const actualValue = element instanceof HTMLElement && element.isContentEditable ? element.textContent || "" : String(element.value ?? "");
  return { selector, value: actualValue, focused: document.activeElement === element, updated: actualValue === value };
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

async function waitForPageCondition(tabId, mode, expected, timeoutMS) {
  const interval = 150;
  const started = Date.now();
  while (Date.now() - started < timeoutMS) {
    const result = await runInTab(tabId, pageConditionMatches, [mode, expected]);
    if (result) return;
    await sleep(interval);
  }
  throw new Error(`Page condition wait timed out: ${mode}`);
}

function pageConditionMatches(mode, expected) {
  if (mode === "text") return String(document.body?.innerText || "").includes(expected);
  if (mode === "url") return location.href.includes(expected);
  if (mode === "title") return document.title.includes(expected);
  const element = document.querySelector(expected);
  if (mode === "selector") return Boolean(element);
  if (!element) return mode === "hidden";
  const rect = element.getBoundingClientRect();
  const style = getComputedStyle(element);
  const visible = rect.width > 0 && rect.height > 0 && style.visibility !== "hidden" && style.display !== "none";
  return mode === "visible" ? visible : !visible;
}

async function acquireDebugger(tabId) {
  const existing = debuggerSessions.get(tabId);
  if (existing) { existing.references++; await existing.ready; return existing.target; }
  const target = { tabId };
  const session = { target, references: 1, networkReferences: 0, cacheDisabledReferences: 0, ready: null };
  session.ready = chrome.debugger.attach(target, "1.3").catch((error) => { if (debuggerSessions.get(tabId) === session) debuggerSessions.delete(tabId); throw error; });
  debuggerSessions.set(tabId, session);
  await session.ready;
  return target;
}

async function releaseDebugger(tabId) {
  const session = debuggerSessions.get(tabId);
  if (!session) return;
  session.references--;
  if (session.references > 0) return;
  debuggerSessions.delete(tabId);
  await chrome.debugger.detach(session.target).catch(() => {});
}

async function withDebugger(tabId, callback) {
  const target = await acquireDebugger(tabId);
  try { return await callback(target); } finally { await releaseDebugger(tabId); }
}

async function acquireNetworkDebugger(tabId, disableCache = false) {
  const target = await acquireDebugger(tabId);
  const session = debuggerSessions.get(tabId);
  if (!session) throw new Error("Debugger session was detached.");
  if (session.networkReferences++ === 0) {
    try {
      await chrome.debugger.sendCommand(target, "Network.enable", { maxTotalBufferSize: 50 * 1024 * 1024, maxResourceBufferSize: 10 * 1024 * 1024 });
    } catch (error) {
      session.networkReferences--;
      await releaseDebugger(tabId);
      throw error;
    }
  }
  if (disableCache && session.cacheDisabledReferences++ === 0) {
    try {
      await chrome.debugger.sendCommand(target, "Network.setCacheDisabled", { cacheDisabled: true });
    } catch (error) {
      session.cacheDisabledReferences--;
      session.networkReferences--;
      if (session.networkReferences === 0) await chrome.debugger.sendCommand(target, "Network.disable").catch(() => {});
      await releaseDebugger(tabId);
      throw error;
    }
  }
  return target;
}

async function releaseNetworkDebugger(tabId, disableCache = false) {
  const session = debuggerSessions.get(tabId);
  if (!session) return;
  if (disableCache) {
    session.cacheDisabledReferences = Math.max(0, session.cacheDisabledReferences - 1);
    if (session.cacheDisabledReferences === 0) await chrome.debugger.sendCommand(session.target, "Network.setCacheDisabled", { cacheDisabled: false }).catch(() => {});
  }
  session.networkReferences = Math.max(0, session.networkReferences - 1);
  if (session.networkReferences === 0) await chrome.debugger.sendCommand(session.target, "Network.disable").catch(() => {});
  await releaseDebugger(tabId);
}

function enqueueInput(tabId, operation) {
  const previous = inputQueues.get(tabId) || Promise.resolve();
  const current = previous.catch(() => {}).then(operation);
  inputQueues.set(tabId, current);
  return current.finally(() => { if (inputQueues.get(tabId) === current) inputQueues.delete(tabId); });
}

async function coordinatesFor(tabId, selector) {
  if (!selector) return runInTab(tabId, () => ({ x: innerWidth / 2, y: innerHeight / 2 }), []);
  return runInTab(tabId, elementCenter, [selector]);
}

async function realClick(tabId, selector, button, clickCount) {
  const point = await coordinatesFor(tabId, selector);
  return withDebugger(tabId, async (target) => {
    await chrome.debugger.sendCommand(target, "Input.dispatchMouseEvent", { type: "mouseMoved", x: point.x, y: point.y, button: "none" });
    await chrome.debugger.sendCommand(target, "Input.dispatchMouseEvent", { type: "mousePressed", x: point.x, y: point.y, button, clickCount });
    await chrome.debugger.sendCommand(target, "Input.dispatchMouseEvent", { type: "mouseReleased", x: point.x, y: point.y, button, clickCount });
    return { selector, x: point.x, y: point.y, button, click_count: clickCount };
  });
}

async function realHover(tabId, selector) {
  const point = await coordinatesFor(tabId, selector);
  return withDebugger(tabId, async (target) => {
    await chrome.debugger.sendCommand(target, "Input.dispatchMouseEvent", { type: "mouseMoved", x: point.x, y: point.y, button: "none" });
    return { selector, x: point.x, y: point.y, hovered: true };
  });
}

async function realType(tabId, selector, text, clear, interval, deadline) {
  await runInTab(tabId, focusElement, [selector]);
  return withDebugger(tabId, async (target) => {
    if (clear) {
      const platform = globalThis.navigator?.platform?.toLowerCase().includes("mac") ? 4 : 2;
      await chrome.debugger.sendCommand(target, "Input.dispatchKeyEvent", { type: "keyDown", key: "a", code: "KeyA", modifiers: platform });
      await chrome.debugger.sendCommand(target, "Input.dispatchKeyEvent", { type: "keyUp", key: "a", code: "KeyA", modifiers: platform });
      await chrome.debugger.sendCommand(target, "Input.dispatchKeyEvent", { type: "keyDown", key: "Backspace", code: "Backspace" });
      await chrome.debugger.sendCommand(target, "Input.dispatchKeyEvent", { type: "keyUp", key: "Backspace", code: "Backspace" });
    }
    let count = 0;
    for (const character of text) {
      if (remaining(deadline) <= 1) throw new Error("Text input timed out.");
      await chrome.debugger.sendCommand(target, "Input.insertText", { text: character });
      count++;
      if (interval) await sleep(Math.min(interval, remaining(deadline)));
    }
    return { selector, characters: count, cleared: clear };
  });
}

async function realKey(tabId, params, deadline) {
  const key = String(params.key || "");
  const code = String(params.code || key);
  const text = String(params.text || "");
  const modifiers = parseModifiers(params.modifiers);
  const repeat = clamp(Number(params.repeat) || 1, 1, 100);
  return withDebugger(tabId, async (target) => {
    for (let index = 0; index < repeat; index++) {
      if (remaining(deadline) <= 1) throw new Error("Key input timed out.");
      await chrome.debugger.sendCommand(target, "Input.dispatchKeyEvent", { type: "keyDown", key, code, modifiers, ...(text ? { text } : {}), autoRepeat: index > 0 });
      await chrome.debugger.sendCommand(target, "Input.dispatchKeyEvent", { type: "keyUp", key, code, modifiers });
    }
    return { key, code, modifiers, repeat };
  });
}

async function realWheel(tabId, selector, deltaX, deltaY) {
  const point = await coordinatesFor(tabId, selector);
  return withDebugger(tabId, async (target) => {
    await chrome.debugger.sendCommand(target, "Input.dispatchMouseEvent", { type: "mouseWheel", x: point.x, y: point.y, deltaX, deltaY });
    return { selector, x: point.x, y: point.y, delta_x: deltaX, delta_y: deltaY };
  });
}

async function createNetworkCapture(tabId, rules, sessionId, disableCache = false) {
  const target = await acquireNetworkDebugger(tabId, disableCache);
  const results = [];
  const requests = new Map();
  const responses = new Map();
  const pending = new Set();
  let sequence = 0;
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
      await releaseNetworkDebugger(tabId, disableCache);
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
  return { tab_id: tab.id, window_id: tab.windowId, index: tab.index ?? 0, url: tab.url || "", title: tab.title || "", status: tab.status || "", active: Boolean(tab.active), pinned: Boolean(tab.pinned), muted: Boolean(tab.mutedInfo?.muted), audible: Boolean(tab.audible), discarded: Boolean(tab.discarded), auto_discardable: tab.autoDiscardable !== false };
}
function windowInfo(window) { return { window_id: window.id, focused: Boolean(window.focused), type: window.type || "normal", state: window.state || "normal", left: window.left, top: window.top, width: window.width, height: window.height, tabs: (window.tabs || []).map(tabInfo) }; }
function groupInfo(group) { return { group_id: group.id, window_id: group.windowId, title: group.title || "", color: group.color || "grey", collapsed: Boolean(group.collapsed) }; }
function requireTab(state) { if (!state.tabId) throw new Error("This browser action requires tab_id."); }
function requireWindow(state) { if (!state.windowId) throw new Error("This browser action requires window_id."); }
function requiredSelector(value) { const selector = String(value || ""); if (!selector || selector.length > 4096) throw new Error("A selector between 1 and 4096 characters is required."); return selector; }
function validateURL(value, allowAbout = false) { const url = new URL(value); if (!(url.protocol === "http:" || url.protocol === "https:" || (allowAbout && url.href === "about:blank"))) throw new Error("Only HTTP and HTTPS URLs are supported."); }
function validGroupColor(value) { return ["grey", "blue", "red", "yellow", "green", "pink", "purple", "cyan", "orange"].includes(value) ? value : "blue"; }
function validWindowState(value) { return ["normal", "minimized", "maximized", "fullscreen"].includes(value) ? value : "normal"; }
function validSameSite(value) { return ["no_restriction", "lax", "strict"].includes(value) ? value : ""; }
function validStorageArea(value) { return value === "session" ? "session" : "local"; }
function validDOMEvent(value) { const event = String(value || ""); if (!["input", "change", "blur", "focus", "submit", "reset"].includes(event)) throw new Error("Unsupported DOM event."); return event; }
function parseModifiers(value) { const flags = { alt: 1, control: 2, ctrl: 2, meta: 4, command: 4, shift: 8 }; return String(value || "").split(",").reduce((result, item) => result | (flags[item.trim().toLowerCase()] || 0), 0); }
function cookieInfo(cookie) { return { name: cookie.name, value: cookie.value, domain: cookie.domain, path: cookie.path, secure: Boolean(cookie.secure), http_only: Boolean(cookie.httpOnly), same_site: cookie.sameSite || "unspecified", session: Boolean(cookie.session), expiration_date: cookie.expirationDate, store_id: cookie.storeId }; }
function cookieURL(cookie) { return `${cookie.secure ? "https" : "http"}://${String(cookie.domain || "").replace(/^\./, "")}${cookie.path || "/"}`; }
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
