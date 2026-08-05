import React, { useEffect, useState } from "react";
import { Monitor, Moon, Sun } from "lucide-react";
import { useTheme } from "../../lib/theme";

const modes = [
  { mode: "light", label: "浅色模式", icon: Sun, tone: "theme-option-light" },
  { mode: "system", label: "跟随系统", icon: Monitor, tone: "theme-option-system" },
  { mode: "dark", label: "深色模式", icon: Moon, tone: "theme-option-dark" }
];

export function ThemeSwitcher() {
  const { mode, setTheme } = useTheme();
  const [expanded, setExpanded] = useState(false);
  const [coarsePointer, setCoarsePointer] = useState(false);
  const activeIndex = Math.max(0, modes.findIndex((item) => item.mode === mode));

  useEffect(() => {
    const media = window.matchMedia("(hover: none), (pointer: coarse)");
    const update = () => setCoarsePointer(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  const select = (next, event) => {
    if (coarsePointer) {
      setTheme(modes[(activeIndex + 1) % modes.length].mode);
      setExpanded(false);
      return;
    }
    event.stopPropagation();
    setTheme(next);
  };

  return <div className="theme-switcher" data-expanded={expanded} role="group" aria-label="外观模式" onMouseEnter={() => !coarsePointer && setExpanded(true)} onMouseLeave={() => !coarsePointer && setExpanded(false)} onFocus={() => !coarsePointer && setExpanded(true)} onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget)) setExpanded(false); }}>
    <div className="theme-switcher-track" style={{ transform: expanded ? "translateX(0)" : `translateX(-${activeIndex * 2}rem)` }}>
      <span className="theme-switcher-indicator" aria-hidden="true" style={{ left: `calc(${activeIndex * 2}rem + .125rem)`, opacity: expanded ? 1 : 0 }} />
      {modes.map((item, index) => { const Icon = item.icon; const active = item.mode === mode; return <button key={item.mode} type="button" className={`theme-switcher-option ${active ? item.tone : "theme-option-muted"}`} aria-label={item.label} aria-pressed={active} title={expanded ? item.label : `${item.label}，点击切换`} tabIndex={expanded || index === activeIndex ? 0 : -1} onClick={(event) => select(item.mode, event)}><Icon size={14} /></button>; })}
    </div>
  </div>;
}
