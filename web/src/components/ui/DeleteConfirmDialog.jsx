import React from "react";
import { Trash2 } from "lucide-react";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "./AlertDialog";

export function DeleteConfirmDialog({ open, onOpenChange, title, description, busy = false, onConfirm, icon: Icon = Trash2, iconSize = 18 }) {
  return <AlertDialog open={open} onOpenChange={onOpenChange}>
    <AlertDialogContent>
      <AlertDialogHeader>
        <div className="alert-dialog-icon"><Icon size={iconSize} /></div>
        <AlertDialogTitle>{title}</AlertDialogTitle>
        <AlertDialogDescription>{description}</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel disabled={busy}>取消</AlertDialogCancel>
        <AlertDialogAction disabled={busy} onClick={onConfirm}>{busy ? "删除中..." : "确认删除"}</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>;
}
