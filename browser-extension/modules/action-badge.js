(function initializeActionBadge(scope) {
  function create(chromeAPI) {
    let last = "";
    return {
      update(connectionState, activeRuns) {
        const count = Math.max(0, Number(activeRuns) || 0);
        const text = count ? (count > 99 ? "99+" : String(count)) : connectionState === "connected" ? "0" : "!";
        const color = count ? "#2563eb" : connectionState === "connected" ? "#71717a" : "#dc2626";
        const signature = `${text}:${color}`;
        if (signature === last) return;
        last = signature;
        void chromeAPI.action.setBadgeText({ text });
        void chromeAPI.action.setBadgeBackgroundColor({ color });
      }
    };
  }
  scope.MeerkitActionBadge = Object.freeze({ create });
})(globalThis);
