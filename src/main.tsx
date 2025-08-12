import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App.tsx";
import "./globals.css";
import { ThemeProvider } from "./components/theme-provider";

const isDevelopment = (import.meta as any).env.DEV;

// Conditionally wrap with StrictMode - disable in dev to prevent duplicate messages
const AppWrapper = isDevelopment ? (
  <ThemeProvider defaultTheme="system" storageKey="ai-ui-theme">
    <App />
  </ThemeProvider>
) : (
  <React.StrictMode>
    <ThemeProvider defaultTheme="system" storageKey="ai-ui-theme">
      <App />
    </ThemeProvider>
  </React.StrictMode>
);

ReactDOM.createRoot(document.getElementById("root")!).render(AppWrapper);
