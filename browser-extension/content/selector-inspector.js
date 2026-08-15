(function installSelectorInspector() {
  const instanceKey = "__meerkitSelectorInspector";
  if (globalThis[instanceKey]) {
    globalThis[instanceKey].enable();
    return;
  }

  let enabled = false;
  let pinned = false;
  let currentElement = null;
  let currentSelector = "";
  let frame = 0;
  let copyTimer = 0;
  const host = document.createElement("div");
  host.dataset.meerkitSelectorInspector = "";
  host.style.cssText = "position:fixed;inset:0;z-index:2147483647;pointer-events:none;contain:layout style;";
  const shadow = host.attachShadow({ mode: "closed" });
  shadow.innerHTML = `
    <style>
      *{box-sizing:border-box} .highlight{position:fixed;display:none;border:2px solid #2563eb;background:rgba(37,99,235,.10);box-shadow:0 0 0 1px rgba(255,255,255,.72) inset;pointer-events:none}
      .tip{position:fixed;display:none;min-width:220px;max-width:min(560px,calc(100vw - 16px));padding:8px;border:1px solid rgba(255,255,255,.18);border-radius:7px;color:#f8fafc;background:#18181b;box-shadow:0 8px 30px rgba(0,0,0,.3);font:12px/1.4 Inter,system-ui,sans-serif;pointer-events:none}
      .tip[data-pinned=true]{border-color:#3b82f6;box-shadow:0 8px 30px rgba(0,0,0,.3),0 0 0 1px rgba(59,130,246,.28);pointer-events:auto}
      .meta{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:5px;color:#a1a1aa;font-size:10px}.meta strong{overflow:hidden;color:#e4e4e7;font-weight:600;text-overflow:ellipsis;white-space:nowrap}
      .selector{display:flex;align-items:center;gap:7px}.selector code{min-width:0;flex:1;overflow:hidden;color:#bfdbfe;font:11px/1.45 ui-monospace,SFMono-Regular,Menlo,monospace;text-overflow:ellipsis;white-space:nowrap}.actions{display:none;flex:none;align-items:center;gap:4px}.tip[data-pinned=true] .actions{display:flex}
      button{height:26px;flex:none;padding:0 8px;border:1px solid #3f3f46;border-radius:4px;color:#f4f4f5;background:#27272a;font:10px/1 Inter,system-ui,sans-serif;cursor:pointer}button:hover{background:#3f3f46}button[data-copied=true]{color:#bbf7d0;border-color:#166534;background:#14532d}.exit{color:#fecaca;border-color:#7f1d1d}.exit:hover{background:#450a0a}
      .hint{margin-top:5px;color:#71717a;font-size:9px}
    </style>
    <div class="highlight"></div>
    <div class="tip" role="status">
      <div class="meta"><strong></strong><span></span></div>
      <div class="selector"><code></code><span class="actions"><button class="copy" type="button">复制</button><button class="resume" type="button">继续检查</button><button class="exit" type="button">退出</button></span></div>
      <div class="hint">单击元素固定 · Ctrl/⌘ + C 快速复制 · Esc 退出</div>
    </div>`;
  const highlight = shadow.querySelector(".highlight");
  const tip = shadow.querySelector(".tip");
  const metaName = shadow.querySelector(".meta strong");
  const metaSize = shadow.querySelector(".meta span");
  const code = shadow.querySelector("code");
  const copyButton = shadow.querySelector(".copy");
  const resumeButton = shadow.querySelector(".resume");
  const exitButton = shadow.querySelector(".exit");
  const hint = shadow.querySelector(".hint");

  function cssEscape(value) {
    if (globalThis.CSS?.escape) return CSS.escape(String(value));
    return String(value).replace(/(^-?\d)|[^a-zA-Z0-9_-]/g, (match, digit) => digit ? `\\3${digit} ` : `\\${match}`);
  }

  function attributeEscape(value) {
    return String(value).replaceAll("\\", "\\\\").replaceAll('"', '\\"');
  }

  function unique(selector) {
    try { return document.querySelectorAll(selector).length === 1; } catch { return false; }
  }

  function stableAttributeSelector(element) {
    for (const name of ["data-testid", "data-test", "data-qa", "name", "aria-label"]) {
      const value = element.getAttribute(name);
      if (!value || value.length > 120) continue;
      const selector = `${element.localName}[${name}="${attributeEscape(value)}"]`;
      if (unique(selector)) return selector;
    }
    return "";
  }

  function selectorSegment(element) {
    let segment = element.localName || "*";
    const classes = [...element.classList].filter((name) => name && name.length < 64 && !/^(active|selected|hover|focus|open|closed|disabled)$/i.test(name)).slice(0, 2);
    if (classes.length) {
      const classSelector = `${segment}${classes.map((name) => `.${cssEscape(name)}`).join("")}`;
      if (unique(classSelector)) return classSelector;
      segment = classSelector;
    }
    const siblings = element.parentElement ? [...element.parentElement.children].filter((node) => node.localName === element.localName) : [];
    if (siblings.length > 1) segment += `:nth-of-type(${siblings.indexOf(element) + 1})`;
    return segment;
  }

  function selectorFor(element) {
    if (!(element instanceof Element)) return "";
    if (element.id) {
      const selector = `#${cssEscape(element.id)}`;
      if (unique(selector)) return selector;
    }
    const attributed = stableAttributeSelector(element);
    if (attributed) return attributed;
    const parts = [];
    let node = element;
    while (node && node !== document.documentElement && parts.length < 6) {
      if (node.id) {
        parts.unshift(`#${cssEscape(node.id)}`);
        break;
      }
      parts.unshift(selectorSegment(node));
      const selector = parts.join(" > ");
      if (unique(selector)) return selector;
      node = node.parentElement;
    }
    return parts.join(" > ") || element.localName;
  }

  function position(element) {
    if (!enabled || !element?.isConnected) return hide();
    const rect = element.getBoundingClientRect();
    if (!rect.width && !rect.height) return hide();
    highlight.style.display = "block";
    highlight.style.left = `${Math.max(0, rect.left)}px`;
    highlight.style.top = `${Math.max(0, rect.top)}px`;
    highlight.style.width = `${Math.max(0, Math.min(innerWidth, rect.right) - Math.max(0, rect.left))}px`;
    highlight.style.height = `${Math.max(0, Math.min(innerHeight, rect.bottom) - Math.max(0, rect.top))}px`;
    tip.style.display = "block";
    const tipRect = tip.getBoundingClientRect();
    let left = Math.min(Math.max(8, rect.left), innerWidth - tipRect.width - 8);
    let top = rect.bottom + 8;
    if (top + tipRect.height > innerHeight - 8) top = Math.max(8, rect.top - tipRect.height - 8);
    tip.style.left = `${left}px`;
    tip.style.top = `${top}px`;
  }

  function inspect(element) {
    if (!element || element === document.documentElement || element === document.body || element === host || host.contains(element)) return;
    currentElement = element;
    currentSelector = selectorFor(element);
    const rect = element.getBoundingClientRect();
    metaName.textContent = `<${element.localName}>`;
    metaSize.textContent = `${Math.round(rect.width)} × ${Math.round(rect.height)}`;
    code.textContent = currentSelector;
    code.title = currentSelector;
    copyButton.dataset.copied = "false";
    copyButton.textContent = "复制";
    position(element);
  }

  function onPointerMove(event) {
    if (!enabled || pinned || event.target === host) return;
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(() => inspect(event.target));
  }

  function onPointerDown(event) {
    if (!enabled || event.target === host) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    if (pinned) return;
    inspect(event.target);
    setPinned(true);
    position(currentElement);
  }

  function blockPageClick(event) {
    if (!enabled || event.target === host) return;
    event.preventDefault();
    event.stopImmediatePropagation();
  }

  function onViewportChange() {
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(() => position(currentElement));
  }

  function onKeyDown(event) {
    if (!enabled) return;
    if (event.key === "Escape") {
      event.preventDefault();
      disable(true);
      return;
    }
    if (event.key.toLowerCase() === "c" && (event.ctrlKey || event.metaKey) && currentSelector) void copySelector(event);
  }

  function hide() {
    highlight.style.display = "none";
    tip.style.display = "none";
  }

  async function copySelector(event) {
    event?.preventDefault();
    event?.stopPropagation();
    event?.stopImmediatePropagation?.();
    if (!currentSelector) return;
    try {
      await navigator.clipboard.writeText(currentSelector);
    } catch {
      const textarea = document.createElement("textarea");
      textarea.value = currentSelector;
      textarea.style.cssText = "position:fixed;left:-9999px;top:0";
      document.documentElement.appendChild(textarea);
      textarea.select(); document.execCommand("copy"); textarea.remove();
    }
    clearTimeout(copyTimer);
    copyButton.dataset.copied = "true";
    copyButton.textContent = "已复制";
    copyTimer = setTimeout(() => { copyButton.dataset.copied = "false"; copyButton.textContent = "复制"; }, 1200);
  }

  function setPinned(value) {
    pinned = Boolean(value);
    tip.dataset.pinned = pinned ? "true" : "false";
    hint.textContent = pinned ? "目标已固定 · 可复制选择器或继续检查 · Esc 退出" : "单击元素固定 · Ctrl/⌘ + C 快速复制 · Esc 退出";
  }

  function resumeInspection(event) {
    event.preventDefault();
    event.stopPropagation();
    setPinned(false);
    currentElement = null;
    currentSelector = "";
    hide();
  }

  function exitInspection(event) {
    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();
    disable(true);
  }

  function enable() {
    if (enabled) return;
    enabled = true;
    if (!host.isConnected) document.documentElement.appendChild(host);
    document.addEventListener("pointermove", onPointerMove, true);
    document.addEventListener("pointerdown", onPointerDown, true);
    document.addEventListener("click", blockPageClick, true);
    document.addEventListener("keydown", onKeyDown, true);
    addEventListener("scroll", onViewportChange, true);
    addEventListener("resize", onViewportChange, true);
  }

  function disable(notify = false) {
    if (!enabled) return;
    enabled = false;
    setPinned(false);
    currentElement = null; currentSelector = "";
    cancelAnimationFrame(frame); clearTimeout(copyTimer); hide();
    document.removeEventListener("pointermove", onPointerMove, true);
    document.removeEventListener("pointerdown", onPointerDown, true);
    document.removeEventListener("click", blockPageClick, true);
    document.removeEventListener("keydown", onKeyDown, true);
    removeEventListener("scroll", onViewportChange, true);
    removeEventListener("resize", onViewportChange, true);
    host.remove();
    if (notify) void chrome.runtime.sendMessage({ type: "debug.inspector.disabled" }).catch(() => {});
  }

  copyButton.addEventListener("click", copySelector);
  resumeButton.addEventListener("click", resumeInspection);
  exitButton.addEventListener("click", exitInspection);
  chrome.runtime.onMessage.addListener((message) => {
    if (message?.type !== "meerkit.selector-inspector") return;
    if (message.enabled === false) disable(); else enable();
  });
  globalThis[instanceKey] = { enable, disable };
  enable();
})();
