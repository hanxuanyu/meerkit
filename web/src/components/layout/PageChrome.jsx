import React, { createContext, useContext, useMemo, useState } from "react";

const PageChromeContext = createContext(null);

export function PageChromeProvider({ children }) {
  const [titleHost, setTitleHost] = useState(null);
  const value = useMemo(() => ({ titleHost, setTitleHost }), [titleHost]);

  return <PageChromeContext.Provider value={value}>{children}</PageChromeContext.Provider>;
}

export function usePageChrome() {
  const context = useContext(PageChromeContext);
  if (!context) throw new Error("usePageChrome must be used within PageChromeProvider");
  return context;
}
