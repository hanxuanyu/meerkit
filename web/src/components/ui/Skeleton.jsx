import React from "react";
import { cn } from "../../lib/utils";

export function Skeleton({ className = "" }) {
  return <span className={cn("skeleton animate-pulse rounded-md bg-muted", className)} />;
}

export function LoadingList({ count = 3 }) {
  return <div className="loading-list">{Array.from({ length: count }, (_, index) => <Skeleton key={index} />)}</div>;
}
