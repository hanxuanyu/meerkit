import { createContext, useContext } from "react";

export const themeStorageKey = "meerkit:theme-mode";
export const ThemeContext = createContext(null);

export function readTheme() {
  try {
    const saved = window.localStorage.getItem(themeStorageKey);
    return saved === "light" || saved === "dark" || saved === "system" ? saved : "system";
  } catch {
    return "system";
  }
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) throw new Error("useTheme must be used within ThemeProvider");
  return context;
}
