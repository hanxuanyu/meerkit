import React from "react";
import { cn } from "../../lib/utils";

export function Card({ children, className = "" }) {
  return <section className={cn("card rounded-lg border bg-card text-card-foreground shadow-sm", className)}>{children}</section>;
}

export function CardHeader({ children, className = "" }) {
  return <div className={cn("section-header flex flex-col space-y-1.5 p-6", className)}>{children}</div>;
}

export function CardTitle({ children, className = "" }) {
  return <h2 className={className}>{children}</h2>;
}

export function CardDescription({ children, className = "" }) {
  return <p className={className}>{children}</p>;
}

export function CardContent({ children, className = "" }) {
  return <div className={cn("p-6 pt-0", className)}>{children}</div>;
}

export function CardFooter({ children, className = "" }) {
  return <div className={cn("modal-footer flex items-center p-6 pt-0", className)}>{children}</div>;
}
