import React, { useEffect, useMemo, useRef, useState } from "react";
import { Braces, Copy, Search } from "lucide-react";
import { toast } from "sonner";
import { getAvailablePlaceholders } from "../../lib/resultSchema";
import { IconButton } from "../ui/IconButton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../ui/Select";

export function PlaceholderHelper({ monitors = [], modules = [] }) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [monitorID, setMonitorID] = useState(monitors[0]?.id || "");
  const helperRef = useRef(null);
  const monitor = monitors.find((item) => item.id === monitorID) || monitors[0];
  const descriptor = modules.find((item) => item.type === monitor?.module_type);
  const placeholders = useMemo(() => getAvailablePlaceholders(descriptor), [descriptor]);
  const filteredMonitors = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return monitors;
    return monitors.filter((item) => `${item.name} ${item.module_type}`.toLowerCase().includes(keyword));
  }, [monitors, search]);

  useEffect(() => {
    if (!open) return undefined;
    const closeOnOutsidePointer = (event) => {
      const target = event.target;
      if (helperRef.current?.contains(target)) return;
      if (target instanceof Element && target.closest("[data-radix-select-content]")) return;
      setOpen(false);
    };
    const closeOnEscape = (event) => { if (event.key === "Escape") setOpen(false); };
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [open]);

  useEffect(() => {
    if (monitorID && monitors.some((item) => item.id === monitorID)) return;
    setMonitorID(monitors[0]?.id || "");
  }, [monitorID, monitors]);

  const copy = async (key) => {
    try {
      await navigator.clipboard.writeText(`{{${key}}}`);
      toast.success("占位符已复制");
    } catch {
      toast.error("无法写入剪贴板");
    }
  };
  return <div className="placeholder-helper" ref={helperRef}><IconButton type="button" variant="outline" size="default" title="插入占位符" aria-label="打开占位符助手" onClick={() => setOpen((current) => !current)}><Braces size={15} /></IconButton>{open && <div className="placeholder-popover"><div className="placeholder-popover-title"><strong>占位符助手</strong><span>选择监控项后复制结果字段</span></div>{monitors.length ? <><div className="placeholder-monitor-search"><Search size={13} /><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索监控项或模块" aria-label="搜索监控项" /></div><Select value={monitor?.id || ""} onValueChange={setMonitorID}><SelectTrigger className="placeholder-select-trigger"><SelectValue placeholder="选择监控项" /></SelectTrigger><SelectContent className="placeholder-select-content">{filteredMonitors.length ? filteredMonitors.map((item) => <SelectItem className="placeholder-select-item" key={item.id} value={item.id}>{item.name}</SelectItem>) : <div className="placeholder-search-empty">没有匹配的监控项</div>}</SelectContent></Select><div className="placeholder-list">{placeholders.map((item) => <button type="button" className="placeholder-item" key={item.key} onClick={() => void copy(item.key)}><span><strong>{item.label}</strong><code>{`{{${item.key}}}`}</code></span><Copy size={13} /></button>)}</div></> : <div className="placeholder-empty">请先创建监控项</div>}</div>}</div>;
}
