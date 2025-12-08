import { ApiErrorHandler } from "./errorHandler.ts";
import { getApiUrl } from "../config.ts";

// Helper to check if the auth cookie exists
const hasAuthCookie = (): boolean => {
  return document.cookie.split(';').some(c => c.trim().startsWith('auth_token='));
};

// Authentication API client for login/logout endpoints
export class AuthAPI {
  constructor() {}

  // POST /login
  async login(token: string): Promise<void> {
    if (!token || token.trim() === "") {
      throw new Error("Authentication token is required");
    }

    return ApiErrorHandler.handleApiCall(async () => {
      const response = await fetch(
        getApiUrl(`/api/login?token=${encodeURIComponent(token.trim())}`),
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
        },
      );

      if (!response.ok) {
        await ApiErrorHandler.handleFetchError(response, "Login");
      }

      // Verify the cookie was actually set by the browser
      // Secure cookies won't be stored on non-HTTPS connections (except localhost)
      if (!hasAuthCookie()) {
        throw new Error(
          "Login succeeded but authentication cookie could not be set. " +
          "This usually happens when accessing the app over HTTP on a non-localhost address. " +
          "Please use HTTPS or access via localhost."
        );
      }
    }, "login");
  }

  // POST /logout
  async logout(): Promise<void> {
    return ApiErrorHandler.handleApiCall(async () => {
      const response = await fetch(getApiUrl("/api/logout"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
      });

      if (!response.ok) {
        await ApiErrorHandler.handleFetchError(response, "Logout");
      }

      // Logout successful - the server should have cleared the cookie
    }, "logout");
  }

  // Check if user is authenticated by making a test request
  async checkAuthStatus(): Promise<boolean> {
    try {
      const response = await fetch(getApiUrl("/api/conversations"), {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
      });

      // If we get a 401 or 403, user is not authenticated
      if (response.status === 401 || response.status === 403) {
        return false;
      }

      // If we get any other error, we can't determine auth status
      if (!response.ok) {
        return false;
      }

      // If request succeeds, user is authenticated
      return true;
    } catch (error) {
      // Network error or other issue - assume not authenticated
      return false;
    }
  }
}

// Default instance
export const authAPI = new AuthAPI();
