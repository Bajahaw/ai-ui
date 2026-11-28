import { afterEach, describe, expect, it, vi } from "vitest";
import { openWidgetInNewTab, WIDGET_SANDBOX } from "./widget-new-tab";

afterEach(() => vi.restoreAllMocks());

describe("openWidgetInNewTab", () => {
  it("uses a sandboxed iframe instead of a same-origin blob", () => {
    const popupDocument = document.implementation.createHTMLDocument("Widget");
    const popup = { document: popupDocument, opener: window } as unknown as Window;
    const open = vi.spyOn(window, "open").mockReturnValue(popup);
    const source = "<script>window.top.secret = 'untrusted'</script>";

    expect(openWidgetInNewTab(source)).toBe(true);
    expect(open).toHaveBeenCalledWith("about:blank", "_blank");
    expect(popup.opener).toBeNull();

    const iframe = popupDocument.querySelector("iframe");
    expect(iframe).not.toBeNull();
    expect(iframe?.srcdoc).toBe(source);
    expect(iframe?.getAttribute("sandbox")).toBe(WIDGET_SANDBOX);
    expect(iframe?.getAttribute("sandbox")).not.toContain("allow-same-origin");
  });

  it("does not create a blob or open a window for empty source", () => {
    const open = vi.spyOn(window, "open");
    expect(openWidgetInNewTab("")).toBe(false);
    expect(open).not.toHaveBeenCalled();
  });
});
