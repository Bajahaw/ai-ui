export const SESSION_ID_KEY = "ai_ui_session_id";

function newSessionId(): string {
  if (typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

export function getSessionId(): string {
  let sessionId = sessionStorage.getItem(SESSION_ID_KEY);
  if (!sessionId) {
    sessionId = newSessionId();
    sessionStorage.setItem(SESSION_ID_KEY, sessionId);
  }
  return sessionId;
}

export function rotateSessionId(): string {
  const sessionId = newSessionId();
  sessionStorage.setItem(SESSION_ID_KEY, sessionId);
  return sessionId;
}

export function getHeaders(init?: HeadersInit): Headers {
  const headers = new Headers(init);
  headers.set("X-Session-ID", getSessionId());

  return headers;
}
