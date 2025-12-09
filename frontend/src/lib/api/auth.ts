import { ApiErrorHandler } from "./errorHandler.ts";
import { getApiUrl } from "../config.ts";
import { AuthStatus, RegisterResponse } from "./types.ts";

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

	// POST /api/auth/register - Register a new instance (returns token)
	async register(): Promise<string> {
		return ApiErrorHandler.handleApiCall(async () => {
			const response = await fetch(getApiUrl("/api/auth/register"), {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
				},
				credentials: "include",
			});

			if (!response.ok) {
				await ApiErrorHandler.handleFetchError(response, "Registration");
			}

			const data: RegisterResponse = await response.json();
			return data.token;
		}, "register");
	}

	// POST /api/auth/login - Login with token
	async login(token: string): Promise<void> {
		if (!token || token.trim() === "") {
			throw new Error("Authentication token is required");
		}

		return ApiErrorHandler.handleApiCall(async () => {
			const formData = new URLSearchParams();
			formData.append('token', token.trim());

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
