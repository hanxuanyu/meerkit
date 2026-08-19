import React from "react";
import { cn } from "../../lib/utils";

export function Label({ children, className = "", ...props }) {
  return <span className={cn("field-label text-sm font-medium leading-none", className)} {...props}>{children}</span>;
}

export const Input = React.forwardRef(function Input({ className = "", ...props }, ref) {
  return <input ref={ref} className={cn("field-control flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50", className)} {...props} />;
});
