import React from "react";
import { cn } from "../../lib/utils";

export function Table({ children, className }) { return <div className="table-wrap"><table className={cn("w-full caption-bottom text-sm", className)}>{children}</table></div>; }
export function TableHeader({ children, className }) { return <thead className={cn("[&_tr]:border-b", className)}>{children}</thead>; }
export function TableBody({ children, className }) { return <tbody className={cn("[&_tr:last-child]:border-0", className)}>{children}</tbody>; }
export function TableRow({ children, className, ...props }) { return <tr className={cn("border-b transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted", className)} {...props}>{children}</tr>; }
export function TableHead({ children, className }) { return <th className={cn("h-10 px-2 text-left align-middle font-medium text-muted-foreground", className)}>{children}</th>; }
export function TableCell({ children, className, ...props }) { return <td className={cn("p-2 align-middle", className)} {...props}>{children}</td>; }
