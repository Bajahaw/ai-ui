import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App.tsx";
import "./globals.css";
import { ThemeProvider } from "./components/theme-provider.tsx";
import { AuthProvider } from "./hooks/useAuth.tsx";
import { ModelsProvider } from "./hooks/useModelsContext.tsx";
import { SettingsDataProvider } from "./hooks/useSettingsData.tsx";
import { ChatGPTOAuthWaitingDialog } from "./components/auth/ChatGPTOAuthWaitingDialog.tsx";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { registerSW } from "virtual:pwa-register";

let refreshing = false;
const reloadForUpdate = () => {
  if (refreshing) return;
  refreshing = true;
  window.location.reload();
};
// Fires when the new precached SW takes control (covers autoUpdate).
navigator.serviceWorker?.addEventListener("controllerchange", reloadForUpdate);

registerSW({
  immediate: true,
  onNeedRefresh: reloadForUpdate,
  onRegisteredSW(_url, r) {
    if (!r) return;
    const poll = () => r.update().catch(() => {});
    const id = setInterval(poll, 60 * 60 * 1000);
    const onVisible = () => {
      if (!document.hidden) poll();
    };
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("focus", poll);
    window.addEventListener("online", poll);
    if (import.meta.hot) {
      import.meta.hot.dispose(() => {
        clearInterval(id);
        document.removeEventListener("visibilitychange", onVisible);
        window.removeEventListener("focus", poll);
        window.removeEventListener("online", poll);
      });
    }
  },
});
/**
 * AuthGuard - Keeps the application shell mounted at all times.
 * Auth-aware hooks decide when to fetch data, and login is opened explicitly
 * from the UI instead of via an automatic route-like switch.
 */
const AuthGuard = ({ children }: { children: React.ReactNode }) => {
  return <>{children}</>;
};

const isDevelopment = (import.meta as any).env.DEV;

// Conditionally wrap with StrictMode - disable in dev to prevent duplicate messages
// AuthProvider must be outermost so auth state is available to guard components
const AppTree = (
  <AuthProvider>
    <ThemeProvider defaultTheme="dark" storageKey="ai-ui-theme">
      <ChatGPTOAuthWaitingDialog />
      <AuthGuard>
        <BrowserRouter>
          <ModelsProvider>
            <SettingsDataProvider>
              <Routes>
                <Route path="/" element={<App />} />
                <Route path="/c/:convId" element={<App />} />
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </SettingsDataProvider>
          </ModelsProvider>
        </BrowserRouter>
      </AuthGuard>
    </ThemeProvider>
  </AuthProvider>
);

const AppWrapper = isDevelopment ? (
  AppTree
) : (
  <React.StrictMode>{AppTree}</React.StrictMode>
);

ReactDOM.createRoot(document.getElementById("root")!).render(AppWrapper);
