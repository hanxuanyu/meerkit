import React from "react";
import { PageHeader } from "../components/layout/PageHeader";
import { BrowserSettings } from "../features/browser/BrowserSettings";

export function BrowserPage() {
  return <div className="page-stack">
    <PageHeader eyebrow="BROWSER AGENTS" title="浏览器" description="管理浏览器执行节点、连接信息与自动化能力。" />
    <BrowserSettings />
  </div>;
}
