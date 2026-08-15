(function initializeMeerkitConfig(scope) {
  scope.MeerkitConfig = Object.freeze({
    protocolVersion: 1,
    defaultSettings: Object.freeze({
      endpoint: "ws://127.0.0.1:8080/api/v1/browser/extension/ws",
      pairingToken: "",
      agentName: "Local Chrome",
      maxConcurrent: 2
    }),
    capabilities: Object.freeze([
      "window.open", "window.focus", "window.state", "window.resize", "window.close",
      "tab.open", "tab.activate", "tab.navigate", "tab.reload", "tab.back", "tab.forward", "tab.duplicate", "tab.move", "tab.pin", "tab.mute", "tab.discard", "tab.auto_discardable", "tab.detect_language", "tab.group", "tab.ungroup", "tab.zoom", "tab.close",
      "page.info", "page.wait", "page.scroll", "page.stop_loading", "page.performance", "page.screenshot",
      "dom.document", "dom.query", "dom.query_all", "dom.focus", "dom.blur", "dom.click", "dom.input", "dom.check", "dom.select", "dom.submit", "dom.set_attribute", "dom.remove_attribute", "dom.dispatch_event", "dom.scroll_into_view",
      "input.click", "input.hover", "input.type", "input.key", "input.wheel",
      "cookie.list", "cookie.set", "cookie.delete", "cookie.clear",
      "storage.get", "storage.set", "storage.remove", "storage.clear",
      "runtime.evaluate", "network.start", "network.stop", "browser.targets", "browser.selector_candidates"
    ]),
    responseChunkSize: 512 * 1024,
    maxResponseSize: 60 * 1024 * 1024,
    maxSocketBuffer: 4 * 1024 * 1024
  });
})(globalThis);
