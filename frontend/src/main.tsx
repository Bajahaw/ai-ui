import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App.tsx";
import "./globals.css";
import { ThemeProvider } from "./components/theme-provider.tsx";
import { AuthProvider } from "./hooks/useAuth.tsx";
import { ModelsProvider } from "./hooks/useModelsContext.tsx";

const isDevelopment = (import.meta as any).env.DEV;

// Conditionally wrap with StrictMode - disable in dev to prevent duplicate messages
const AppWrapper = isDevelopment ? (
  <ModelsProvider>
    <AuthProvider>
      <ThemeProvider defaultTheme="dark" storageKey="ai-ui-theme">
        <App />
      </ThemeProvider>
    </AuthProvider>
  </ModelsProvider>
) : (
  <React.StrictMode>
    <ModelsProvider>
      <AuthProvider>
        <ThemeProvider defaultTheme="dark" storageKey="ai-ui-theme">
          <App />
        </ThemeProvider>
      </AuthProvider>
    </ModelsProvider>
  </React.StrictMode>
);

ReactDOM.createRoot(document.getElementById("root")!).render(AppWrapper);
