import React from "react";
import { Activity, Bell, ChartNoAxesColumnIncreasing, ChevronsRight, CircleX, History, Inbox, Layers3, RefreshCw, Settings2, X } from "lucide-react";
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from "../ui/ContextMenu";
import { IconButton } from "../ui/IconButton";

const tabDefinitions = {
  overview: { title: "总览", icon: Activity, closable: false },
  monitors: { title: "监控项", icon: Layers3, closable: true },
  statusBoard: { title: "状态看板", icon: ChartNoAxesColumnIncreasing, closable: true },
  notifications: { title: "通知渠道", icon: Bell, closable: true },
  inbox: { title: "通知中心", icon: Inbox, closable: true },
  settings: { title: "系统设置", icon: Settings2, closable: true }
};

function getTabDefinition(id, recordTabs) {
  if (tabDefinitions[id]) return tabDefinitions[id];
  if (id.startsWith("monitor-details:") || id.startsWith("monitor-records:")) { const context = recordTabs[id]; const monitor = context?.monitor || context; return { title: monitor?.name ? `${monitor.name} · 详情` : "监控详情", icon: History, closable: true }; }
  return null;
}

export function WorkspaceTabs({ tabs, activeId, onActivate, onClose, onRefresh, onCloseOthers, onCloseRight, recordTabs = {} }) {
  return <div className="workspace-tabs"><div className="workspace-tabs-scroll"><div className="workspace-tabs-list" role="tablist" aria-label="已打开的页面">{tabs.map((id) => {
    const tab = getTabDefinition(id, recordTabs);
    if (!tab) return null;
    const Icon = tab.icon;
    const active = activeId === id;
    const hasOtherClosable = tabs.some((item) => item !== id && getTabDefinition(item, recordTabs)?.closable);
    const hasRightClosable = tabs.slice(tabs.indexOf(id) + 1).some((item) => getTabDefinition(item, recordTabs)?.closable);
    return <ContextMenu key={id}><ContextMenuTrigger asChild><div className="workspace-tab" data-active={active} onMouseDown={(event) => { if (event.button === 1) event.preventDefault(); }} onAuxClick={(event) => { if (event.button === 1 && tab.closable) { event.preventDefault(); onClose(id); } }}><button type="button" role="tab" aria-selected={active} className="workspace-tab-trigger" onClick={() => onActivate(id)}><Icon size={14} /><span title={tab.title}>{tab.title}</span></button>{tab.closable && <IconButton className="workspace-tab-close" size="sm" title={`关闭${tab.title}`} aria-label={`关闭${tab.title}`} onClick={(event) => { event.stopPropagation(); onClose(id); }}><X size={13} /></IconButton>}</div></ContextMenuTrigger><ContextMenuContent><ContextMenuItem onSelect={() => onRefresh(id)}><RefreshCw size={15} />刷新页签</ContextMenuItem><ContextMenuSeparator /><ContextMenuItem disabled={!tab.closable} onSelect={() => onClose(id)}><X size={15} />关闭页签</ContextMenuItem><ContextMenuItem disabled={!hasOtherClosable} onSelect={() => onCloseOthers(id)}><CircleX size={15} />关闭其他页签</ContextMenuItem><ContextMenuItem disabled={!hasRightClosable} onSelect={() => onCloseRight(id)}><ChevronsRight size={15} />关闭右侧页签</ContextMenuItem></ContextMenuContent></ContextMenu>;
  })}</div></div><div className="workspace-tabs-status">{tabs.length} 个页面</div></div>;
}
