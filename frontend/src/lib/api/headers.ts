export const SESSION_ID_KEY = "ai_ui_session_id";
export const ACCESS_TOKEN_KEY = "ai_ui_access_token";
export const ACTIVE_PROFILE_KEY = "ai-ui-profile";

// randomId falls back to Math.random when WebCrypto is unavailable.
// crypto.randomUUID() requires a secure context, so plain-HTTP hosts
// (LAN IP deployments) would otherwise crash every API call.
function randomId(): string {
  try {
    const c = globalThis.crypto;
    if (c && typeof c.randomUUID === "function") {
      return c.randomUUID();
    }
    if (c && typeof c.getRandomValues === "function") {
      const b = new Uint8Array(16);
      c.getRandomValues(b);
      b[6] = (b[6] & 0x0f) | 0x40;
      b[8] = (b[8] & 0x3f) | 0x80;
      const h = Array.from(b, (x) => x.toString(16).padStart(2, "0")).join(
        "",
      );
      return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
    }
  } catch {
    // fall through to Math.random
  }
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (ch) => {
    const r = Math.floor(Math.random() * 16);
    const v = ch === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export function getSessionId(): string {
  let sessionId = sessionStorage.getItem(SESSION_ID_KEY);
  if (!sessionId) {
    sessionId = randomId();
    sessionStorage.setItem(SESSION_ID_KEY, sessionId);
  }
  return sessionId;
}

export function rotateSessionId(): string {
  const sessionId = randomId();
  sessionStorage.setItem(SESSION_ID_KEY, sessionId);
  return sessionId;
}

export function getHeaders(init?: HeadersInit): Headers {
  const headers = new Headers(init);
  headers.set("X-Session-ID", getSessionId());

  return headers;
}

// ---- Profiles-mode access token (for <img>/<video>/<a> resource loads) ----

export function getAccessToken(): string | null {
  return sessionStorage.getItem(ACCESS_TOKEN_KEY);
}

export function setAccessToken(token: string | null): void {
  if (token) {
    sessionStorage.setItem(ACCESS_TOKEN_KEY, token);
  } else {
    sessionStorage.removeItem(ACCESS_TOKEN_KEY);
  }
}

/**
 * Appends ?access_token= to local /data/* URLs so resource loads that cannot
 * carry cookies (notably inside the mobile WebView) still resolve the user.
 * External URLs and URLs without a token are returned unchanged.
 */
export function withAccessToken(url: string): string {
  if (!url || !url.startsWith("/data/")) {
    return url;
  }
  const token = getAccessToken();
  if (!token) {
    return url;
  }
  return `${url}${url.includes("?") ? "&" : "?"}access_token=${encodeURIComponent(token)}`;
}
