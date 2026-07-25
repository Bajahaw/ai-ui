import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useRef,
  ReactNode,
} from "react";
import { authAPI } from "@/lib/api/auth.ts";

interface AuthContextType {
  isAuthenticated: boolean;
  isCheckingAuth: boolean;
  isLoading: boolean;
  /** True while waiting for ChatGPT OAuth (popup or manual paste). */
  chatgptOAuthPending: boolean;
  login: (username: string, password: string) => Promise<void>;
  loginWithChatGPT: () => Promise<{ providerId?: string; model?: string }>;
  /** Paste the localhost:1455 redirect URL when automatic callback fails (remote deploy). */
  submitChatGPTCallbackUrl: (url: string) => Promise<void>;
  /** Abort an in-flight ChatGPT OAuth attempt (waiting modal closed, etc.). */
  cancelChatGPTOAuth: () => void;
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
  const [chatgptOAuthPending, setChatgptOAuthPending] = useState(false);
  const oauthAbortRef = useRef<AbortController | null>(null);

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
    const abort = new AbortController();
    oauthAbortRef.current = abort;

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

    const throwIfCancelled = () => {
      if (abort.signal.aborted) {
        throw new Error("ChatGPT sign-in cancelled");
      }
    };

    try {
      setError(null);
      setIsLoading(true);
      setChatgptOAuthPending(true);
      const { auth_url, state } = await authAPI.startChatGPTLogin();
      throwIfCancelled();

      popup = window.open(auth_url, "chatgpt-oauth", "width=520,height=720");
      if (!popup) {
        throw new Error(
          "ChatGPT sign-in popup was blocked. Allow popups for this site and try again.",
        );
      }
      window.addEventListener("message", onMessage);

      const deadline = Date.now() + 5 * 60 * 1000;
      // Keep waiting even if the OAuth window is closed — on mobile users often
      // leave that window to copy/paste the redirect URL into "Having problems?".
      // Only cancel when the waiting modal is closed (abort), success, error, or timeout.
      while (Date.now() < deadline) {
        await new Promise((r) => setTimeout(r, 1200));
        throwIfCancelled();

        const status = await authAPI.pollChatGPTLogin(state);
        throwIfCancelled();

        if (status.status === "pending") {
          continue;
        }

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
      // Cancellation from the waiting dialog is intentional — don't sticky-error the login form.
      if (errorMessage !== "ChatGPT sign-in cancelled") {
        setError(errorMessage);
      }
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
      if (oauthAbortRef.current === abort) {
        oauthAbortRef.current = null;
      }
      setChatgptOAuthPending(false);
      setIsLoading(false);
    }
  };

  /** Finish OAuth when the browser lands on localhost:1455 and the user pastes that URL. */
  const submitChatGPTCallbackUrl = async (url: string): Promise<void> => {
    try {
      await authAPI.submitChatGPTCallback(url);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Invalid callback URL";
      throw new Error(errorMessage);
    }
  };

  const cancelChatGPTOAuth = () => {
    oauthAbortRef.current?.abort();
  };

  const clearError = () => {
    setError(null);
  };

  const value: AuthContextType = {
    isAuthenticated,
    isCheckingAuth,
    isLoading,
    chatgptOAuthPending,
    login,
    loginWithChatGPT,
    submitChatGPTCallbackUrl,
    cancelChatGPTOAuth,
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
