import React from "react";
import { createRoot } from "react-dom/client";
import { ThemeProvider } from "./components/theme/ThemeProvider";
import { FloatingScrollbars } from "./components/ui/FloatingScrollbars";
import { AuthGate } from "./features/auth/AuthGate";
import "./styles.css";

createRoot(document.getElementById("root")).render(<ThemeProvider><AuthGate /><FloatingScrollbars /></ThemeProvider>);
