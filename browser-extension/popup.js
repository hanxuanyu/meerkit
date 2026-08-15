import { initializeDebugMode } from "./popup/debug-mode.js";
import { initializeStatus } from "./popup/status.js";

document.querySelector("#settings").addEventListener("click", () => chrome.runtime.openOptionsPage());
initializeStatus();
initializeDebugMode();
