export interface ScreenshotSource {
  data: ArrayBuffer;
  filename: string;
  mimeType?: string;
}

export interface ScreenshotOptions {
  /** One-based page, slide, or worksheet number. */
  page?: number;
  /** Maximum output dimensions in pixels (1–2048). */
  width?: number;
  height?: number;
  /** One-based Excel viewport; defaults to A1:H20. */
  range?: { row: number; col: number; rows: number; cols: number };
}

export type ResolvedScreenshotOptions = Required<ScreenshotOptions>;

export type ScreenshotRenderer = (
  source: ScreenshotSource,
  options: ResolvedScreenshotOptions,
  signal: AbortSignal,
) => Promise<Blob>;
