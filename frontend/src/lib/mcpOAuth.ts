import { startMCPOAuth } from "@/lib/api/mcpServers";

/** Must match mcpOAuthBroadcastName in backend/cmd/tools/mcp_oauth.go. */
export const MCP_OAUTH_CHANNEL = "mcp-oauth";
const WAIT_TIMEOUT_MS = 10 * 60 * 1000;

interface MCPOAuthMessage {
  type: "mcp-oauth";
  serverId: string;
  error: string;
}

const isOAuthMessage = (data: unknown): data is MCPOAuthMessage =>
  typeof data === "object" &&
  data !== null &&
  (data as MCPOAuthMessage).type === "mcp-oauth";

/**
 * Opens a blank popup. Call this synchronously inside the click handler,
 * before any await, or the browser will block it.
 */
export const openOAuthPopup = (): Window | null =>
  window.open("about:blank", "mcp-oauth", "popup,width=520,height=720");

/**
 * Resolves when the callback page reports success for serverId. The result
 * arrives over BroadcastChannel, since many providers set
 * Cross-Origin-Opener-Policy and that cuts the popup off from window.opener.
 */
export const waitForMCPOAuth = (
  serverId: string,
  timeoutMs = WAIT_TIMEOUT_MS,
): Promise<void> =>
  new Promise((resolve, reject) => {
    const channel =
      typeof BroadcastChannel !== "undefined"
        ? new BroadcastChannel(MCP_OAUTH_CHANNEL)
        : null;

    const cleanup = () => {
      clearTimeout(timer);
      channel?.close();
      window.removeEventListener("message", onWindowMessage);
    };
    const handle = (data: unknown) => {
      if (!isOAuthMessage(data) || data.serverId !== serverId) return;
      cleanup();
      if (data.error) reject(new Error(data.error));
      else resolve();
    };
    const onWindowMessage = (e: MessageEvent) => {
      if (e.origin === window.location.origin) handle(e.data);
    };

    const timer = setTimeout(() => {
      cleanup();
      reject(new Error("Timed out waiting for authorization"));
    }, timeoutMs);
    if (channel) channel.onmessage = (e) => handle(e.data);
    window.addEventListener("message", onWindowMessage);
  });

/** Runs the OAuth flow for a saved server in the given popup. */
export const authorizeMCPServer = async (
  serverId: string,
  popup: Window | null,
): Promise<void> => {
  let authURL: string;
  try {
    authURL = await startMCPOAuth(serverId);
  } catch (err) {
    popup?.close();
    throw err;
  }
  const win =
    popup && !popup.closed
      ? popup
      : window.open(authURL, "mcp-oauth", "popup,width=520,height=720");
  if (!win) {
    throw new Error("The authorization popup was blocked. Allow popups for this site and try again.");
  }
  win.location.href = authURL;
  await waitForMCPOAuth(serverId);
};
