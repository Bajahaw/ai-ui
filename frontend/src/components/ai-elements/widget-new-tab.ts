export const WIDGET_SANDBOX =
  "allow-scripts allow-forms allow-modals allow-downloads";

/** Open untrusted widget HTML only inside an opaque-origin sandboxed frame. */
export function openWidgetInNewTab(srcdoc: string): boolean {
  if (!srcdoc) return false;

  const popup = window.open("about:blank", "_blank");
  if (!popup) return false;
  popup.opener = null;

  const document = popup.document;
  document.title = "Widget";
  document.body.style.margin = "0";

  const iframe = document.createElement("iframe");
  iframe.title = "Widget";
  iframe.setAttribute("sandbox", WIDGET_SANDBOX);
  iframe.srcdoc = srcdoc;
  iframe.style.cssText = "display:block;width:100%;height:100vh;border:0";
  document.body.appendChild(iframe);
  return true;
}
