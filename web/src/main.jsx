import React from "react";
import { createRoot } from "react-dom/client";
import { App } from "./app/App";
import { ThemeProvider } from "./components/theme/ThemeProvider";
import "./styles.css";

createRoot(document.getElementById("root")).render(<ThemeProvider><App /></ThemeProvider>);
