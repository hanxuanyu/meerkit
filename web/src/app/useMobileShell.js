import { useCallback, useEffect, useState } from "react";

export function useMobileShell() {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const [mobileTopbarHidden, setMobileTopbarHidden] = useState(false);

  const toggleNavigation = useCallback(() => {
    if (window.matchMedia("(max-width: 640px)").matches) setMobileSidebarOpen((current) => !current);
    else setSidebarCollapsed((current) => !current);
  }, []);
  const closeMobileSidebar = useCallback(() => setMobileSidebarOpen(false), []);
  const resetMobileNavigation = useCallback(() => {
    setMobileSidebarOpen(false);
    setMobileTopbarHidden(false);
  }, []);

  useEffect(() => {
    let lastScrollY = window.scrollY;
    let frame = 0;
    const updateTopbar = () => {
      frame = 0;
      if (!window.matchMedia("(max-width: 640px)").matches) {
        setMobileTopbarHidden(false);
        lastScrollY = window.scrollY;
        return;
      }
      const nextScrollY = Math.max(window.scrollY, 0);
      const delta = nextScrollY - lastScrollY;
      if (nextScrollY <= 8) setMobileTopbarHidden(false);
      else if (delta > 8) setMobileTopbarHidden(true);
      else if (delta < -8) setMobileTopbarHidden(false);
      if (Math.abs(delta) > 8) lastScrollY = nextScrollY;
    };
    const onScroll = () => { if (!frame) frame = window.requestAnimationFrame(updateTopbar); };
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    return () => {
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onScroll);
      if (frame) window.cancelAnimationFrame(frame);
    };
  }, []);

  return { sidebarCollapsed, mobileSidebarOpen, mobileTopbarHidden, toggleNavigation, closeMobileSidebar, resetMobileNavigation };
}
