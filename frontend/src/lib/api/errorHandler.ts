// Error handling utilities for API calls

export interface ApiError extends Error {
  status?: number;
  statusText?: string;
  url?: string;
}

export class ApiErrorHandler {
  /**
   * Wraps an API call with comprehensive error handling
   */
  static async handleApiCall<T>(
    apiCall: () => Promise<T>,
    context: string,
  ): Promise<T> {
    try {
      return await apiCall();
    } catch (error) {
      console.error(`API Error in ${context}:`, error);

      // If it's already a properly formatted error, re-throw it
      if (error instanceof Error) {
        throw error;
      }

      // Handle unexpected error types
      if (typeof error === "string") {
        throw new Error(`${context}: ${error}`);
      }

      // Handle unknown error types
      throw new Error(`${context}: Unknown error occurred`);
    }
  }

  /**
   * Creates a standardized API error with additional context
   */
  static createApiError(
    message: string,
    status?: number,
    statusText?: string,
    url?: string,
  ): ApiError {
    const error = new Error(message) as ApiError;
    error.name = "ApiError";
    error.status = status;
    error.statusText = statusText;
    error.url = url;
    return error;
  }

  /**
   * Handles fetch response errors with detailed information
   */
  static async handleFetchError(
    response: Response,
    context: string,
  ): Promise<never> {
    let errorDetails: string;

    try {
      const errorText = await response.text();
      errorDetails = errorText || response.statusText;
    } catch {
      errorDetails = response.statusText || "Unknown error";
    }

    const message = `${context} failed (${response.status}): ${errorDetails}`;

    throw this.createApiError(
      message,
      response.status,
      response.statusText,
      response.url,
    );
  }

  /**
   * Validates that a response contains expected data structure
   */
  static validateResponse<T>(
    data: unknown,
    validator: (data: unknown) => data is T,
    context: string,
  ): T {
    if (!validator(data)) {
      throw new Error(`${context}: Invalid response data structure`);
    }
    return data;
  }

  /**
   * Checks if error is a network connectivity issue
   */
  static isNetworkError(error: unknown): boolean {
    if (error instanceof TypeError && error.message.includes("fetch")) {
      return true;
    }

    if (error instanceof Error) {
      const message = error.message.toLowerCase();
      return (
        message.includes("network") ||
        message.includes("connection") ||
        message.includes("timeout") ||
        message.includes("offline")
      );
    }

    return false;
  }

  /**
   * Gets user-friendly error message from API error
   */
  static getUserFriendlyMessage(error: unknown): string {
    if (this.isNetworkError(error)) {
      return "Network connection error. Please check your internet connection and try again.";
    }

    if (error instanceof Error) {
      // Remove technical details for user-facing messages
      const message = error.message;

      // Handle specific HTTP status codes
      if (message.includes("(400)")) {
        return "Invalid request. Please try again.";
      }
      if (message.includes("(401)")) {
        return "Authentication required. Please log in and try again.";
      }
      if (message.includes("(403)")) {
        return "Access denied. You may not have permission for this action.";
      }
      if (message.includes("(404)")) {
        return "The requested resource was not found.";
      }
      if (message.includes("(429)")) {
        return "Too many requests. Please wait a moment and try again.";
      }
      if (
        message.includes("(500)") ||
        message.includes("(502)") ||
        message.includes("(503)")
      ) {
        return "Server error. Please try again later.";
      }

      // Return the original message if it's already user-friendly
      if (!message.includes("(") && !message.includes("Error:")) {
        return message;
      }
    }

    return "An unexpected error occurred. Please try again.";
  }
}

export const isConversationArray = (data: unknown): data is Array<any> => {
  return Array.isArray(data) && data.every(isConversation);
};

export const isConversation = (data: unknown): data is any => {
  return (
    data !== null &&
    typeof data === "object" &&
    "id" in data &&
    "createdAt" in data &&
    "updatedAt" in data &&
    typeof (data as any).id === "string" &&
    typeof (data as any).createdAt === "string" &&
    typeof (data as any).updatedAt === "string"
  );
};

export const isChatResponse = (data: unknown): data is any => {
  return (
    data !== null &&
    typeof data === "object" &&
    "messages" in data &&
    isMessagesMap((data as any).messages)
  );
};

export const isMessagesMap = (data: unknown): data is Record<number, any> => {
  if (data === null || typeof data !== "object") return false;

  const entries = Object.entries(data as Record<string, any>);

  for (const [key, value] of entries) {
    // keys should be numeric (stringified numbers)
    if (Number.isNaN(Number(key))) return false;

    if (
      value === null ||
      typeof value !== "object" ||
      !("id" in value) ||
      typeof (value as any).id !== "number" ||
      !("role" in value) ||
      typeof (value as any).role !== "string" ||
      !("content" in value) ||
      typeof (value as any).content !== "string"
    ) {
      return false;
    }

    // convId is expected from backend, when present must be string
    if (
      "convId" in (value as any) &&
      typeof (value as any).convId !== "string"
    ) {
      return false;
    }

    // Optional parentId must be a number when present
    if (
      "parentId" in (value as any) &&
      typeof (value as any).parentId !== "number"
    ) {
      return false;
    }

    // Optional children must be an array of numbers when present
    if ("children" in (value as any)) {
      const children = (value as any).children;
      if (
        !Array.isArray(children) ||
        !children.every((c: any) => typeof c === "number")
      ) {
        return false;
      }
    }

    // Optional attachment must be string when present
    if (
      "attachment" in (value as any) &&
      typeof (value as any).attachment !== "string"
    ) {
      return false;
    }
  }
  return true;
};
