import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  ReactNode,
} from "react";
import { authAPI } from "@/lib/api/auth.ts";

interface AuthContextType {
  isAuthenticated: boolean;
  isCheckingAuth: boolean;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<void>;
  loginWithChatGPT: () => Promise<{ providerId?: string; model?: string }>;
  logout: () => Promise<void>;
  register: (username: string, password: string) => Promise<void>;
  error: string | null;
  clearError: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

interface AuthProviderProps {
  children: ReactNode;
}

export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isCheckingAuth, setIsCheckingAuth] = useState(true);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Check authentication status on mount
  useEffect(() => {
    const checkAuth = async () => {
      try {
        const status = await authAPI.getAuthStatus();
        setIsAuthenticated(status.authenticated);
      } catch (err) {
        console.error("Error checking auth status:", err);
        setIsAuthenticated(false);
      } finally {
        setIsCheckingAuth(false);
      }
    };

    checkAuth();
  }, []);

  const register = async (
    username: string,
    password: string,
  ): Promise<void> => {
    try {
      setError(null);
      setIsLoading(true);
      await authAPI.register(username, password);
      await login(username, password);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Registration failed";
      setError(errorMessage);
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  const login = async (username: string, password: string) => {
    try {
      setError(null);
      setIsLoading(true);
      await authAPI.login(username, password);
      // After login, re-check auth status
      const status = await authAPI.getAuthStatus();
      if (!status.authenticated) {
        throw new Error(
          "Login succeeded but user is not authenticated, this is usually happen due to using secure cookie in a non-secure context. Please use HTTPS or access the app via localhost.",
        );
      }
      setIsAuthenticated(true);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : "Login failed";
      setError(errorMessage);
      setIsAuthenticated(false);
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  const logout = async () => {
    try {
      setError(null);
      setIsLoading(true);
      await authAPI.logout();
    } catch (err) {
      // A 401 means the session was already gone on the server — still log out locally.
      console.error(
        "Logout API error (ignored, clearing local auth state anyway):",
        err,
      );
    } finally {
      setIsAuthenticated(false);
      setIsLoading(false);
    }
  };

  /** Open ChatGPT OAuth, poll until done, create session + ChatGPT provider. */
  const loginWithChatGPT = async (): Promise<{
    providerId?: string;
    model?: string;
  }> => {
    let popup: Window | null = null;
    const onMessage = (event: MessageEvent) => {
      // Callback server is localhost:1455 only — ignore other origins.
      const origin = event?.origin ?? "";
      if (
        origin !== "http://127.0.0.1:1455" &&
        origin !== "http://localhost:1455"
      ) {
        return;
      }
      if (event?.data?.type === "chatgpt-oauth" && popup && !popup.closed) {
        try {
          popup.close();
        } catch {
          /* ignore */
        }
      }
    };
    try {
      setError(null);
      setIsLoading(true);
      const { auth_url, state } = await authAPI.startChatGPTLogin();
      popup = window.open(auth_url, "chatgpt-oauth", "width=520,height=720");
      window.addEventListener("message", onMessage);

      const deadline = Date.now() + 5 * 60 * 1000;
      while (Date.now() < deadline) {
        await new Promise((r) => setTimeout(r, 1200));
        const status = await authAPI.pollChatGPTLogin(state);
        if (status.status === "pending") continue;
        if (status.status === "error") {
          throw new Error(status.error || "ChatGPT sign-in failed");
        }
        // Close the OAuth popup as soon as the backend confirms success.
        if (popup && !popup.closed) {
          try {
            popup.close();
          } catch {
            /* ignore */
          }
        }
        // success — sign-in mode sets cookie on the status response;
        // connect mode (already logged in) keeps the existing session.
        const authStatus = await authAPI.getAuthStatus();
        if (authStatus.authenticated) {
          setIsAuthenticated(true);
        } else if (!isAuthenticated) {
          throw new Error(
            "ChatGPT sign-in succeeded but session cookie was not set. Use HTTPS or access via localhost.",
          );
        }
        return {
          providerId: status.provider_id,
          model: status.model,
        };
      }
      throw new Error("ChatGPT sign-in timed out");
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "ChatGPT sign-in failed";
      setError(errorMessage);
      throw err;
    } finally {
      window.removeEventListener("message", onMessage);
      if (popup && !popup.closed) {
        try {
          popup.close();
        } catch {
          /* ignore */
        }
      }
      setIsLoading(false);
    }
  };

  const clearError = () => {
    setError(null);
  };

  const value: AuthContextType = {
    isAuthenticated,
    isCheckingAuth,
    isLoading,
    login,
    loginWithChatGPT,
    logout,
    register,
    error,
    clearError,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuth = (): AuthContextType => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
};
