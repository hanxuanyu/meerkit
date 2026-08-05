import React from "react";
import { Toaster as SonnerToaster } from "sonner";
import { useTheme } from "../../lib/theme";

export function Toaster(props) {
  const { resolvedTheme } = useTheme();
  return <SonnerToaster theme={resolvedTheme} toastOptions={{ classNames: { toast: "toast border-border bg-background text-foreground shadow-lg", description: "text-muted-foreground", actionButton: "bg-primary text-primary-foreground", cancelButton: "bg-muted text-muted-foreground" } }} {...props} />;
}
