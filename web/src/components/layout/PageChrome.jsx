import React, { createContext, useContext, useMemo, useState } from "react";

const PageChromeContext = createContext(null);

export function PageChromeProvider({ children, activePage = "" }) {
  const [titleHost, setTitleHost] = useState(null);
  const value = useMemo(() => ({ titleHost, setTitleHost, activePage, pageKey: "" }), [activePage, titleHost]);

  return <PageChromeContext.Provider value={value}>{children}</PageChromeContext.Provider>;
}

export function PageChromeScope({ pageKey, children }) {
  const parent = usePageChrome();
  const value = useMemo(() => ({ ...parent, pageKey }), [pageKey, parent]);
  return <PageChromeContext.Provider value={value}>{children}</PageChromeContext.Provider>;
}

export function usePageChrome() {
  const context = useContext(PageChromeContext);
  if (!context) throw new Error("usePageChrome must be used within PageChromeProvider");
  return context;
}
