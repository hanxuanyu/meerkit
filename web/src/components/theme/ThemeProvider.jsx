import React, { useCallback, useEffect, useMemo, useState } from "react";
import { readTheme, ThemeContext, themeStorageKey } from "../../lib/theme";

export function ThemeProvider({ children }) {
  const [mode, setMode] = useState(readTheme);
  const [systemDark, setSystemDark] = useState(() => window.matchMedia("(prefers-color-scheme: dark)").matches);
  const resolvedTheme = mode === "system" ? (systemDark ? "dark" : "light") : mode;

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = (event) => setSystemDark(event.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  useEffect(() => {
    const update = (event) => { if (event.key === themeStorageKey) setMode(readTheme()); };
    window.addEventListener("storage", update);
    return () => window.removeEventListener("storage", update);
  }, []);

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle("dark", resolvedTheme === "dark");
    root.dataset.theme = mode;
    root.style.colorScheme = resolvedTheme;
    document.querySelector('meta[name="theme-color"]')?.setAttribute("content", resolvedTheme === "dark" ? "#18191d" : "#fafafa");
  }, [mode, resolvedTheme]);

  const setTheme = useCallback((next) => {
    setMode(next);
    try { window.localStorage.setItem(themeStorageKey, next); } catch { /* Keep the in-memory selection. */ }
  }, []);

  const value = useMemo(() => ({ mode, resolvedTheme, setTheme }), [mode, resolvedTheme, setTheme]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
