import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useRef,
  ReactNode,
} from "react";
import { authAPI } from "@/lib/api/auth.ts";
import {
  ACTIVE_PROFILE_KEY,
  setAccessToken,
} from "@/lib/api/headers.ts";
import { clearAuthCache, readAuthCache, writeAuthCache } from "@/lib/authCache";
import { conversationCache } from "@/lib/conversationCache";

interface AuthContextType {
  isAuthenticated: boolean;
  isCheckingAuth: boolean;
  isLoading: boolean;
  /** Whether new account creation is allowed (password register + new ChatGPT users). */
  registrationEnabled: boolean;
  /** "password" (server) or "profiles" (passwordless local mode). */
  authMode: string;
  isProfilesMode: boolean;
  /** Local profiles (profiles mode only). */
  profiles: string[];
  /** Currently selected profile (profiles mode only). */
  activeProfile: string | null;
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
  /** Select a local profile (profiles mode only). */
  selectProfile: (username: string) => Promise<void>;
  /** Create a local profile and select it (profiles mode only). */
  createProfile: (username: string) => Promise<void>;
  /** Delete a local profile and its data (profiles mode only). */
  deleteProfile: (username: string) => Promise<void>;
  error: string | null;
  clearError: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

interface AuthProviderProps {
  children: ReactNode;
}

export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(
    () => readAuthCache()?.authenticated === true,
  );
  const [isCheckingAuth, setIsCheckingAuth] = useState(true);
  const [isLoading, setIsLoading] = useState(false);
  const [registrationEnabled, setRegistrationEnabled] = useState(true);
  const [authMode, setAuthMode] = useState("password");
  const [profiles, setProfiles] = useState<string[]>([]);
  const [activeProfile, setActiveProfile] = useState<string | null>(() => {
    try {
      return localStorage.getItem(ACTIVE_PROFILE_KEY);
    } catch {
      return null;
    }
  });
  const isProfilesMode = authMode === "profiles";
  const [error, setError] = useState<string | null>(null);
  const [chatgptOAuthPending, setChatgptOAuthPending] = useState(false);
  const oauthAbortRef = useRef<AbortController | null>(null);

  const applyProfileSession = async (username: string) => {
    const res = await authAPI.selectProfile(username);
    setAccessToken(res.access_token);
    try {
      localStorage.setItem(ACTIVE_PROFILE_KEY, res.username);
    } catch {
      // storage unavailable — session still works for this tab
    }
    setActiveProfile(res.username);
    setProfiles((prev) =>
      prev.includes(res.username) ? prev : [...prev, res.username],
    );
    await conversationCache.clear();
    clearAuthCache();
    writeAuthCache({ authenticated: true });
    setIsAuthenticated(true);
  };

  const ensureProfileSession = async () => {
    const list = await authAPI.listProfiles();
    const names = list.map((p) => p.username);
    setProfiles(names);
    let stored: string | null = null;
    try {
      stored = localStorage.getItem(ACTIVE_PROFILE_KEY);
    } catch {
      stored = null;
    }
    const pick =
      stored && names.includes(stored) ? stored : names.length > 0 ? names[0] : null;
    if (pick) {
      await applyProfileSession(pick);
      return;
    }
    // First run: create the default profile (hooks seed settings + MCP).
    const created = await authAPI.createProfile("Default");
    setProfiles([created.username]);
    setAccessToken(created.access_token);
    try {
      localStorage.setItem(ACTIVE_PROFILE_KEY, created.username);
    } catch {
      // ignore
    }
    setActiveProfile(created.username);
    await conversationCache.clear();
    clearAuthCache();
    writeAuthCache({ authenticated: true });
    setIsAuthenticated(true);
  };

  useEffect(() => {
    const applySignedOut = async () => {
      setIsAuthenticated(false);
      setAccessToken(null);
      clearAuthCache();
      await conversationCache.clear();
    };

    const checkAuth = async () => {
      try {
        const status = await authAPI.getAuthStatus();
        const mode = status.auth_mode || "password";
        setAuthMode(mode);
        setRegistrationEnabled(status.registration_enabled !== false);
        if (mode === "profiles") {
          // Passwordless mode: silently (re)select a profile — no login UI.
          await ensureProfileSession();
          return;
        }
        if (status.authenticated) {
          setIsAuthenticated(true);
          writeAuthCache({ authenticated: true });
        } else {
          await applySignedOut();
        }
      } catch (err) {
        if (readAuthCache()?.authenticated) {
          console.warn("Auth check did not confirm sign-out; keeping session", err);
          setIsAuthenticated(true);
        } else {
          console.error("Error checking auth status:", err);
          await applySignedOut();
        }
      } finally {
        setIsCheckingAuth(false);
      }
    };

    void checkAuth();
    window.addEventListener("online", checkAuth);
    return () => window.removeEventListener("online", checkAuth);
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
      await conversationCache.clear();
      clearAuthCache();
      writeAuthCache({ authenticated: true });
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
      setAccessToken(null);
      clearAuthCache();
      await conversationCache.clear();
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
          if (!isAuthenticated) {
            await conversationCache.clear();
            clearAuthCache();
          }
          setIsAuthenticated(true);
          writeAuthCache({ authenticated: true });
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

  const selectProfile = async (username: string): Promise<void> => {
    try {
      setError(null);
      setIsLoading(true);
      await applyProfileSession(username);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Could not switch profile";
      setError(errorMessage);
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  const createProfile = async (username: string): Promise<void> => {
    try {
      setError(null);
      setIsLoading(true);
      const created = await authAPI.createProfile(username);
      setProfiles((prev) =>
        prev.includes(created.username)
          ? prev
          : [...prev, created.username],
      );
      await applyProfileSession(created.username);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Could not create profile";
      setError(errorMessage);
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  const deleteProfile = async (username: string): Promise<void> => {
    try {
      setError(null);
      setIsLoading(true);
      await authAPI.deleteProfile(username);
      setProfiles((prev) => prev.filter((p) => p !== username));
      if (activeProfile === username) {
        // Fall back to another profile, or sign out to the picker.
        const remaining = profiles.filter((p) => p !== username);
        if (remaining.length > 0) {
          await applyProfileSession(remaining[0]);
        } else {
          try {
            localStorage.removeItem(ACTIVE_PROFILE_KEY);
          } catch {
            // ignore
          }
          setActiveProfile(null);
          setIsAuthenticated(false);
          setAccessToken(null);
          clearAuthCache();
          await conversationCache.clear();
        }
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Could not delete profile";
      setError(errorMessage);
      throw err;
    } finally {
      setIsLoading(false);
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
    registrationEnabled,
    authMode,
    isProfilesMode,
    profiles,
    activeProfile,
    chatgptOAuthPending,
    login,
    loginWithChatGPT,
    submitChatGPTCallbackUrl,
    cancelChatGPTOAuth,
    logout,
    register,
    selectProfile,
    createProfile,
    deleteProfile,
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
