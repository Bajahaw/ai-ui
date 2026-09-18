export function parseSandboxOrigin(raw: string | undefined | null): string {
  const trimmed = (raw || "").trim();
  if (!trimmed) return "";
  try {
    const url = new URL(trimmed);
    if (url.protocol !== "http:" && url.protocol !== "https:") return "";
    if (url.username || url.password) return "";
    if (!url.hostname) return "";
    return url.origin;
  } catch {
    return "";
  }
}

export function resolveSandboxFrameOrigin(
  raw: string | undefined | null,
  pageOrigin = typeof window !== "undefined" ? window.location.origin : "",
): string {
  const origin = parseSandboxOrigin(raw);
  if (!origin || (pageOrigin && origin === pageOrigin)) return "";
  return origin;
}

let cached: Promise<string> | null = null;

export function resetSandboxOriginCache(): void {
  cached = null;
}

export async function getSandboxFrameOrigin(): Promise<string> {
  if (!cached) {
    cached = fetch("/api/version", { cache: "no-store" })
      .then((res) => (res.ok ? res.json() : {}))
      .then((data: { sandbox_origin?: string }) =>
        resolveSandboxFrameOrigin(data.sandbox_origin),
      )
      .catch(() => "");
  }
  return cached;
}
