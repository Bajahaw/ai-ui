import { getHeaders } from "./headers";

/**
 * Fetch read-aloud audio for an assistant message via GET /api/tts/messages/{id}.
 *
 * GET + ETag/Cache-Control lets the browser cache the MP3 so repeated plays
 * of the same message (with the same TTS settings) skip re-synthesis.
 */
export async function synthesizeMessageSpeech(
  messageId: string | number,
  signal?: AbortSignal,
): Promise<Blob> {
  const id = encodeURIComponent(String(messageId));
  const response = await fetch(`/api/tts/messages/${id}`, {
    method: "GET",
    headers: getHeaders(),
    credentials: "include",
    // Allow HTTP cache (ETag / Cache-Control from server).
    cache: "default",
    signal,
  });

  if (!response.ok) {
    let detail = response.statusText;
    try {
      const text = await response.text();
      if (text) detail = text;
    } catch {
      // ignore
    }
    throw new Error(detail || `TTS failed (${response.status})`);
  }

  return response.blob();
}
