import React, { useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { cn } from "../../lib/utils";
import { IconButton } from "./IconButton";
import { Input } from "./Input";

export const PasswordInput = React.forwardRef(function PasswordInput({ className = "", disabled, ...props }, ref) {
  const [visible, setVisible] = useState(false);
  const VisibilityIcon = visible ? EyeOff : Eye;
  const label = visible ? "隐藏密码" : "显示密码";

  return <div className="password-input">
    <Input ref={ref} {...props} className={cn("password-input-control", className)} disabled={disabled} type={visible ? "text" : "password"} />
    <IconButton className="password-input-toggle" variant="ghost" size="sm" disabled={disabled} title={label} aria-label={label} aria-pressed={visible} onMouseDown={(event) => event.preventDefault()} onClick={() => setVisible((current) => !current)}><VisibilityIcon size={14} /></IconButton>
  </div>;
});
