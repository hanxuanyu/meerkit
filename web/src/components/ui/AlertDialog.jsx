import React from "react";
import * as AlertDialogPrimitive from "@radix-ui/react-alert-dialog";
import { cn } from "../../lib/utils";
import { Button } from "./Button";

export const AlertDialog = AlertDialogPrimitive.Root;
export const AlertDialogTrigger = AlertDialogPrimitive.Trigger;

export function AlertDialogContent({ className, children, ...props }) {
  return <AlertDialogPrimitive.Portal><AlertDialogPrimitive.Overlay className="alert-dialog-overlay fixed inset-0 z-50 duration-200 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 motion-reduce:duration-0" /><AlertDialogPrimitive.Content className={cn("alert-dialog-content fixed left-1/2 top-1/2 z-50 grid w-full max-w-[calc(100%-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background shadow-lg outline-none duration-200 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 motion-reduce:duration-0 sm:max-w-lg", className)} {...props}>{children}</AlertDialogPrimitive.Content></AlertDialogPrimitive.Portal>;
}

export function AlertDialogHeader({ className, ...props }) {
  return <div className={cn("alert-dialog-header", className)} {...props} />;
}

export function AlertDialogFooter({ className, ...props }) {
  return <div className={cn("alert-dialog-footer", className)} {...props} />;
}

export function AlertDialogTitle({ className, ...props }) {
  return <AlertDialogPrimitive.Title className={cn("alert-dialog-title", className)} {...props} />;
}

export function AlertDialogDescription({ className, ...props }) {
  return <AlertDialogPrimitive.Description className={cn("alert-dialog-description", className)} {...props} />;
}

export function AlertDialogAction({ className, ...props }) {
  return <Button asChild variant="destructive" className={className}><AlertDialogPrimitive.Action {...props} /></Button>;
}

export function AlertDialogCancel({ className, ...props }) {
  return <Button asChild variant="outline" className={className}><AlertDialogPrimitive.Cancel {...props} /></Button>;
}
