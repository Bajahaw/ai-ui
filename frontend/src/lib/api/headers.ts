
export const SESSION_ID_KEY = "ai_ui_session_id";

export function getSessionId(): string {
    let sessionId = sessionStorage.getItem(SESSION_ID_KEY);
    if (!sessionId) {
        sessionId = crypto.randomUUID();
        sessionStorage.setItem(SESSION_ID_KEY, sessionId);
    }
    return sessionId;
}

export function getHeaders(init?: HeadersInit): Headers {
    const headers = new Headers(init);
    headers.set("X-Session-ID", getSessionId());
    
    return headers;
}
