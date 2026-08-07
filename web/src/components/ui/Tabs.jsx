import React, { createContext, useContext, useState } from "react";
import { cn } from "../../lib/utils";

const TabsContext = createContext(null);

export function Tabs({ value, defaultValue, onValueChange, className, children }) {
  const [uncontrolledValue, setUncontrolledValue] = useState(defaultValue || "");
  const activeValue = value ?? uncontrolledValue;
  const changeValue = (nextValue) => {
    if (value === undefined) setUncontrolledValue(nextValue);
    onValueChange?.(nextValue);
  };
  return <TabsContext.Provider value={{ activeValue, changeValue }}><div className={cn("tabs", className)}>{children}</div></TabsContext.Provider>;
}

export function TabsList({ className, children }) {
  return <div role="tablist" className={cn("tabs-list", className)}>{children}</div>;
}

export function TabsTrigger({ value, className, children, ...props }) {
  const context = useContext(TabsContext);
  const active = context?.activeValue === value;
  return <button type="button" role="tab" aria-selected={active} tabIndex={active ? 0 : -1} className={cn("tabs-trigger", className)} data-state={active ? "active" : "inactive"} onClick={() => context?.changeValue(value)} {...props}>{children}</button>;
}

export function TabsContent({ value, className, children, forceMount = false }) {
  const context = useContext(TabsContext);
  const active = context?.activeValue === value;
  if (!active && !forceMount) return null;
  return <div role="tabpanel" hidden={!active} className={cn("tabs-content", className)}>{children}</div>;
}
