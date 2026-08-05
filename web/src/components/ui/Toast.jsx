import React from "react";
import { Toaster as SonnerToaster } from "sonner";
import { useTheme } from "../../lib/theme";

export function Toaster(props) {
  const { resolvedTheme } = useTheme();
  return <SonnerToaster {...props} theme={resolvedTheme} closeButton={false} toastOptions={{ classNames: { description: "text-muted-foreground", actionButton: "bg-primary text-primary-foreground", cancelButton: "bg-muted text-muted-foreground" } }} />;
}
