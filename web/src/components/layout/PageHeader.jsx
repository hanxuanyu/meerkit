import React from "react";
import { createPortal } from "react-dom";
import { usePageChrome } from "./PageChrome";

export function PageHeader({ eyebrow, title, description }) {
  const { titleHost } = usePageChrome();

  return <>
    {titleHost && createPortal(<div className="topbar-page-copy">
      <div className="topbar-page-title-line"><h1>{title}</h1>{eyebrow && <span>{eyebrow}</span>}</div>
      {description && <p title={description}>{description}</p>}
    </div>, titleHost)}
  </>;
}
