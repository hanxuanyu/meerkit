import React, { useEffect, useRef } from "react";

const hideDelay = 600;
const edgeInset = 3;
const minThumbSize = 28;

export function FloatingScrollbars() {
  const verticalRef = useRef(null);
  const horizontalRef = useRef(null);

  useEffect(() => {
    let frame = 0;
    let hideTimer = 0;
    let activeTarget = null;

    const hide = () => {
      if (verticalRef.current) verticalRef.current.dataset.visible = "false";
      if (horizontalRef.current) horizontalRef.current.dataset.visible = "false";
    };

    const viewportMetrics = () => {
      const fixedTopbar = document.querySelector(".topbar");
      const topbarStyle = fixedTopbar ? window.getComputedStyle(fixedTopbar) : null;
      const topbarRect = topbarStyle?.position === "fixed" ? fixedTopbar.getBoundingClientRect() : null;
      return {
        rect: { top: Math.max(0, topbarRect?.bottom || 0), right: window.innerWidth, bottom: window.innerHeight, left: 0 },
        clientWidth: window.innerWidth,
        clientHeight: window.innerHeight,
        scrollWidth: document.scrollingElement?.scrollWidth || window.innerWidth,
        scrollHeight: document.scrollingElement?.scrollHeight || window.innerHeight,
        scrollLeft: document.scrollingElement?.scrollLeft || window.scrollX,
        scrollTop: document.scrollingElement?.scrollTop || window.scrollY
      };
    };

    const getMetrics = (target) => {
      if (target === document || target === document.documentElement || target === document.body || target === document.scrollingElement) return viewportMetrics();
      if (!(target instanceof Element)) return null;
      const targetRect = target.getBoundingClientRect();
      const rect = { top: targetRect.top, right: targetRect.right, bottom: targetRect.bottom, left: targetRect.left };
      let ancestor = target.parentElement;
      while (ancestor) {
        const style = window.getComputedStyle(ancestor);
        if ([style.overflow, style.overflowX, style.overflowY].some((value) => ["auto", "scroll", "hidden", "clip"].includes(value))) {
          const ancestorRect = ancestor.getBoundingClientRect();
          rect.top = Math.max(rect.top, ancestorRect.top);
          rect.right = Math.min(rect.right, ancestorRect.right);
          rect.bottom = Math.min(rect.bottom, ancestorRect.bottom);
          rect.left = Math.max(rect.left, ancestorRect.left);
        }
        ancestor = ancestor.parentElement;
      }
      if (target.classList.contains("page-content")) {
        const paddingTop = Number.parseFloat(window.getComputedStyle(target).paddingTop) || 0;
        rect.top = Math.min(rect.bottom, rect.top + paddingTop);
      }
      return {
        rect,
        clientWidth: target.clientWidth,
        clientHeight: target.clientHeight,
        scrollWidth: target.scrollWidth,
        scrollHeight: target.scrollHeight,
        scrollLeft: target.scrollLeft,
        scrollTop: target.scrollTop
      };
    };

    const render = () => {
      frame = 0;
      const metrics = getMetrics(activeTarget);
      const vertical = verticalRef.current;
      const horizontal = horizontalRef.current;
      if (!metrics || !vertical || !horizontal) return;

      const top = Math.max(0, metrics.rect.top) + edgeInset;
      const right = Math.min(window.innerWidth, metrics.rect.right) - edgeInset;
      const bottom = Math.min(window.innerHeight, metrics.rect.bottom) - edgeInset;
      const left = Math.max(0, metrics.rect.left) + edgeInset;
      const trackHeight = Math.max(0, bottom - top);
      const trackWidth = Math.max(0, right - left);
      const hasVerticalOverflow = metrics.scrollHeight > metrics.clientHeight + 1 && trackHeight > minThumbSize;
      const hasHorizontalOverflow = metrics.scrollWidth > metrics.clientWidth + 1 && trackWidth > minThumbSize;

      if (hasVerticalOverflow) {
        const thumbHeight = Math.max(minThumbSize, trackHeight * metrics.clientHeight / metrics.scrollHeight);
        const travel = trackHeight - thumbHeight;
        const progress = metrics.scrollTop / Math.max(1, metrics.scrollHeight - metrics.clientHeight);
        vertical.style.top = `${top + travel * progress}px`;
        vertical.style.left = `${right - 4}px`;
        vertical.style.height = `${thumbHeight}px`;
        vertical.dataset.visible = "true";
      } else {
        vertical.dataset.visible = "false";
      }

      if (hasHorizontalOverflow) {
        const thumbWidth = Math.max(minThumbSize, trackWidth * metrics.clientWidth / metrics.scrollWidth);
        const travel = trackWidth - thumbWidth;
        const progress = metrics.scrollLeft / Math.max(1, metrics.scrollWidth - metrics.clientWidth);
        horizontal.style.top = `${bottom - 4}px`;
        horizontal.style.left = `${left + travel * progress}px`;
        horizontal.style.width = `${thumbWidth}px`;
        horizontal.dataset.visible = "true";
      } else {
        horizontal.dataset.visible = "false";
      }

      window.clearTimeout(hideTimer);
      hideTimer = window.setTimeout(hide, hideDelay);
    };

    const handleScroll = (event) => {
      activeTarget = event.target === document ? document.scrollingElement : event.target;
      if (!frame) frame = window.requestAnimationFrame(render);
    };
    const handleResize = () => {
      hide();
      activeTarget = null;
    };

    document.addEventListener("scroll", handleScroll, true);
    window.addEventListener("resize", handleResize);
    return () => {
      document.removeEventListener("scroll", handleScroll, true);
      window.removeEventListener("resize", handleResize);
      window.cancelAnimationFrame(frame);
      window.clearTimeout(hideTimer);
    };
  }, []);

  return <div className="floating-scrollbars" aria-hidden="true"><span ref={verticalRef} className="floating-scrollbar is-vertical" data-visible="false" /><span ref={horizontalRef} className="floating-scrollbar is-horizontal" data-visible="false" /></div>;
}
