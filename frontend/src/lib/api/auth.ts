import { ApiErrorHandler } from "./errorHandler.ts";
import { getApiUrl } from "../config.ts";
import { AuthStatus } from "./types.ts";

// Authentication API client
export class AuthAPI {
	constructor() { }

	// GET /api/auth/status - Check registration and authentication status
	async getAuthStatus(): Promise<AuthStatus> {
		return ApiErrorHandler.handleApiCall(async () => {
			const response = await fetch(getApiUrl("/api/auth/status"), {
				method: "GET",
				headers: {
					"Content-Type": "application/json",
				},
				credentials: "include",
			});

			// Status endpoint returns different status codes:
			// 200: registered and authenticated
			// 401: registered but not authenticated
			// 403: not registered

			if (!response.ok && response.status !== 401 && response.status !== 403) {
				await ApiErrorHandler.handleFetchError(response, "Auth Status Check");
			}

			const data: AuthStatus = await response.json();
			return data;
		}, "getAuthStatus");
	}

	// POST /api/auth/register - Register a new instance
	async register(username: string, password: string): Promise<void> {
		if (!username || !password) {
			throw new Error("Username and password are required");
		}

		return ApiErrorHandler.handleApiCall(async () => {
			const response = await fetch(getApiUrl("/api/auth/register"), {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
				},
				body: JSON.stringify({ username, password }),
				credentials: "include",
			});

			if (!response.ok) {
				await ApiErrorHandler.handleFetchError(response, "Registration");
			}
		}, "register");
	}

	// POST /api/auth/login - Login with username and password
	async login(username: string, password: string): Promise<void> {
		if (!username || !password) {
			throw new Error("Username and password are required");
		}

		return ApiErrorHandler.handleApiCall(async () => {
			const formData = new URLSearchParams();
			formData.append('username', username);
			formData.append('password', password);

			const response = await fetch(getApiUrl("/api/auth/login"), {
				method: "POST",
				headers: {
					"Content-Type": "application/x-www-form-urlencoded",
				},
				body: formData,
				credentials: "include",
			});

			if (!response.ok) {
				await ApiErrorHandler.handleFetchError(response, "Login");
			}
		}, "login");
	}

	// POST /api/auth/logout - Logout and clear cookie
	async logout(): Promise<void> {
		return ApiErrorHandler.handleApiCall(async () => {
			const response = await fetch(getApiUrl("/api/auth/logout"), {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
				},
				credentials: "include",
			});

			if (!response.ok) {
				await ApiErrorHandler.handleFetchError(response, "Logout");
			}
		}, "logout");
	}
}

// Default instance
export const authAPI = new AuthAPI();
