import { createContext, useContext, useEffect, useState } from "react";
import {
  ACCENT_SETTING_KEY,
  ACCENT_STORAGE_KEY,
  type AccentId,
  applyAccentToDocument,
  resolveAccent,
} from "@/lib/accent";
import { useSettings } from "@/hooks/useSettings";

type Theme = "dark" | "light";

type ThemeProviderProps = {
  children: React.ReactNode;
  defaultTheme?: Theme;
  storageKey?: string;
};

type ThemeProviderState = {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  accent: AccentId;
  setAccent: (accent: AccentId) => void;
};

const initialState: ThemeProviderState = {
  theme: "dark",
  setTheme: () => null,
  accent: "neutral",
  setAccent: () => null,
};

const ThemeProviderContext = createContext<ThemeProviderState>(initialState);

export function ThemeProvider({
  children,
  defaultTheme = "dark",
  storageKey = "vite-ui-theme",
  ...props
}: ThemeProviderProps) {
  const [theme, setTheme] = useState<Theme>(() => {
    const stored = localStorage.getItem(storageKey) as Theme;
    if (stored && (stored === "light" || stored === "dark")) {
      return stored;
    }
    return defaultTheme;
  });

  // The backend is the source of truth for the accent; localStorage is only a
  // boot cache so the first paint already has the right colour.
  const [accent, setAccent] = useState<AccentId>(() =>
    resolveAccent(localStorage.getItem(ACCENT_STORAGE_KEY)),
  );
  const { settings } = useSettings();
  const remoteAccent = settings[ACCENT_SETTING_KEY];

  useEffect(() => {
    if (remoteAccent !== undefined) setAccent(resolveAccent(remoteAccent));
  }, [remoteAccent]);

  useEffect(() => {
    applyAccentToDocument(accent);
    localStorage.setItem(ACCENT_STORAGE_KEY, accent);
  }, [accent]);

  useEffect(() => {
    const root = window.document.documentElement;

    root.classList.remove("light", "dark");
    root.classList.add(theme);
    root.style.colorScheme = theme;

    // Mobile browser / PWA chrome (status + nav bars) follow theme-color, not CSS.
    const applyChromeColor = () => {
      const bg = getComputedStyle(document.body).backgroundColor;
      const meta = document.querySelector('meta[name="theme-color"]');
      if (meta && bg) meta.setAttribute("content", bg);

      const apple = document.querySelector(
        'meta[name="apple-mobile-web-app-status-bar-style"]',
      );
      if (apple) {
        apple.setAttribute(
          "content",
          theme === "dark" ? "black-translucent" : "default",
        );
      }
    };

    // Wait a frame so theme CSS variables are applied before reading background.
    const id = requestAnimationFrame(applyChromeColor);
    return () => cancelAnimationFrame(id);
  }, [theme]);

  const value = {
    theme,
    setTheme: (theme: Theme) => {
      localStorage.setItem(storageKey, theme);
      setTheme(theme);
    },
    accent,
    setAccent,
  };

  return (
    <ThemeProviderContext.Provider {...props} value={value}>
      {children}
    </ThemeProviderContext.Provider>
  );
}

export const useTheme = () => {
  const context = useContext(ThemeProviderContext);

  if (context === undefined)
    throw new Error("useTheme must be used within a ThemeProvider");

  return context;
};
