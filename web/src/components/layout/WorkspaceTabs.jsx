import React from "react";
import { Activity, Bell, ChevronsRight, CircleX, Layers3, RefreshCw, Settings2, X } from "lucide-react";
import { Button } from "../ui/Button";
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from "../ui/ContextMenu";

const tabDefinitions = {
  overview: { title: "总览", icon: Activity, closable: false },
  monitors: { title: "监控项", icon: Layers3, closable: true },
  notifications: { title: "通知渠道", icon: Bell, closable: true },
  settings: { title: "系统设置", icon: Settings2, closable: true }
};

export function WorkspaceTabs({ tabs, activeId, onActivate, onClose, onRefresh, onCloseOthers, onCloseRight }) {
  return <div className="workspace-tabs"><div className="workspace-tabs-scroll"><div className="workspace-tabs-list" role="tablist" aria-label="已打开的页面">{tabs.map((id) => {
    const tab = tabDefinitions[id];
    if (!tab) return null;
    const Icon = tab.icon;
    const active = activeId === id;
    const hasOtherClosable = tabs.some((item) => item !== id && tabDefinitions[item]?.closable);
    const hasRightClosable = tabs.slice(tabs.indexOf(id) + 1).some((item) => tabDefinitions[item]?.closable);
    return <ContextMenu key={id}><ContextMenuTrigger asChild><div className="workspace-tab" data-active={active} onMouseDown={(event) => { if (event.button === 1) event.preventDefault(); }} onAuxClick={(event) => { if (event.button === 1 && tab.closable) { event.preventDefault(); onClose(id); } }}><button type="button" role="tab" aria-selected={active} className="workspace-tab-trigger" onClick={() => onActivate(id)}><Icon size={14} /><span>{tab.title}</span></button>{tab.closable && <Button type="button" variant="ghost" size="icon" className="workspace-tab-close" title={`关闭${tab.title}`} onClick={(event) => { event.stopPropagation(); onClose(id); }}><X size={13} /></Button>}</div></ContextMenuTrigger><ContextMenuContent><ContextMenuItem onSelect={() => onRefresh(id)}><RefreshCw size={15} />刷新页签</ContextMenuItem><ContextMenuSeparator /><ContextMenuItem disabled={!tab.closable} onSelect={() => onClose(id)}><X size={15} />关闭页签</ContextMenuItem><ContextMenuItem disabled={!hasOtherClosable} onSelect={() => onCloseOthers(id)}><CircleX size={15} />关闭其他页签</ContextMenuItem><ContextMenuItem disabled={!hasRightClosable} onSelect={() => onCloseRight(id)}><ChevronsRight size={15} />关闭右侧页签</ContextMenuItem></ContextMenuContent></ContextMenu>;
  })}</div></div><div className="workspace-tabs-status">{tabs.length} 个页面</div></div>;
}
