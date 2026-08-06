import { createContext, useContext, useEffect, useState } from "react";

type Theme = "dark" | "light";

type ThemeProviderProps = {
  children: React.ReactNode;
  defaultTheme?: Theme;
  storageKey?: string;
};

type ThemeProviderState = {
  theme: Theme;
  setTheme: (theme: Theme) => void;
};

const initialState: ThemeProviderState = {
  theme: "dark",
  setTheme: () => null,
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
