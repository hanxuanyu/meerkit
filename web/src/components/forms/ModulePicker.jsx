import React, { useMemo, useRef, useState } from "react";
import { Check, ChevronsUpDown, Package, Search } from "lucide-react";
import { cn } from "../../lib/utils";
import { Input } from "../ui/Input";
import { Popover, PopoverContent, PopoverTrigger } from "../ui/Popover";

function groupModules(modules, search) {
  const keyword = search.trim().toLocaleLowerCase();
  const filtered = keyword ? modules.filter((module) => [module.name, module.type, module.description, module.plugin_name, module.plugin_id].some((value) => String(value || "").toLocaleLowerCase().includes(keyword))) : modules;
  const groups = new Map();
  filtered.forEach((module) => {
    const id = module.plugin_id || "other";
    const label = module.plugin_name || (id === "system" ? "系统内置" : module.plugin_id || "其他模块");
    if (!groups.has(id)) groups.set(id, { id, label, modules: [] });
    groups.get(id).modules.push(module);
  });
  return [...groups.values()]
    .sort((left, right) => left.label.localeCompare(right.label, "zh-CN"))
    .map((group) => ({ ...group, modules: group.modules.sort((left, right) => (left.name || left.type).localeCompare(right.name || right.type, "zh-CN")) }));
}

export function ModulePicker({ modules = [], value = "", onValueChange, id, "aria-labelledby": ariaLabelledby }) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const inputRef = useRef(null);
  const selected = modules.find((module) => module.type === value);
  const groups = useMemo(() => groupModules(modules, search), [modules, search]);
  const select = (moduleType) => {
    onValueChange?.(moduleType);
    setOpen(false);
    setSearch("");
  };
  return <Popover modal open={open} onOpenChange={(nextOpen) => { setOpen(nextOpen); if (!nextOpen) setSearch(""); }}>
    <PopoverTrigger asChild><button id={id} type="button" className="module-picker-trigger" aria-labelledby={ariaLabelledby} aria-haspopup="listbox" aria-expanded={open}>
      <span className="module-picker-value"><strong>{selected?.name || selected?.type || "选择采集模块"}</strong>{selected && <small>{selected.plugin_name || selected.plugin_id || "其他模块"}</small>}</span><ChevronsUpDown size={15} aria-hidden="true" />
    </button></PopoverTrigger>
    <PopoverContent className="module-picker-popover" align="start" onPointerDown={(event) => event.stopPropagation()} onOpenAutoFocus={(event) => { event.preventDefault(); inputRef.current?.focus(); }}>
      <div className="module-picker-search"><Search size={14} aria-hidden="true" /><Input ref={inputRef} value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索模块名称、类型或插件" aria-label="搜索采集模块" /></div>
      <div className="module-picker-list" role="listbox" aria-label="采集模块">
        {groups.length ? groups.map((group) => <section className="module-picker-group" key={group.id} role="group" aria-label={group.label}>
          <div className="module-picker-group-label"><Package size={13} aria-hidden="true" /><span><strong>{group.label}</strong>{group.id !== group.label && <small>{group.id}</small>}</span><em>{group.modules.length}</em></div>
          {group.modules.map((module) => <button type="button" role="option" aria-selected={module.type === value} className={cn("module-picker-option", module.type === value && "is-selected")} key={module.type} onClick={() => select(module.type)}>
            <span><strong>{module.name || module.type}</strong><small>{module.type}{module.description ? ` · ${module.description}` : ""}</small></span>{module.type === value && <Check size={14} aria-hidden="true" />}
          </button>)}
        </section>) : <div className="module-picker-empty">没有匹配的采集模块</div>}
      </div>
    </PopoverContent>
  </Popover>;
}
