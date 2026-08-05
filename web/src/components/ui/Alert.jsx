import React from "react";
import { cn } from "../../lib/utils";

export function Alert({ children, className = "" }) { return <div className={cn("alert relative w-full rounded-lg border p-4", className)}>{children}</div>; }
export function AlertTitle({ children }) { return <h5 className="mb-1 font-medium leading-none tracking-tight">{children}</h5>; }
export function AlertDescription({ children }) { return <div className="text-sm text-muted-foreground">{children}</div>; }
