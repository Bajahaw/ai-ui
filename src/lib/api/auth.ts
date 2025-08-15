import { ApiErrorHandler } from "./errorHandler";
import { getApiUrl } from "../config";

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

      // Login successful - the server should have set the secure cookie
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
