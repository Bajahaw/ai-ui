import React, { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog.tsx";
import { useAuth } from "@/hooks/useAuth.tsx";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils.ts";

const AppLogo = ({ className }: { className?: string }) => (
  <svg
    className={className}
    width="40"
    height="40"
    viewBox="0 0 1191 1191"
    xmlns="http://www.w3.org/2000/svg"
    aria-hidden
  >
    <circle
      cx="595.276"
      cy="614.849"
      r="499.517"
      className="fill-foreground"
    />
    <path
      d="M924.054,572.425c0,82.98 -73.269,158.521 -188.149,193.982l-341.54,105.426l-112.51,-235.507c-9.883,-20.687 -14.91,-42.231 -14.91,-63.901c0,-118.419 147.22,-214.56 328.554,-214.56c181.334,0 328.554,96.141 328.554,214.56Z"
      className="fill-background"
    />
  </svg>
);

const ChatGPTLogo = ({ className }: { className?: string }) => (
  <svg
    className={className}
    width="40"
    height="40"
    viewBox="0 0 24 24"
    fill="currentColor"
    aria-hidden
  >
    <path d="M22.282 9.821a5.985 5.985 0 0 0-.516-4.91 6.046 6.046 0 0 0-6.51-2.9A6.065 6.065 0 0 0 4.981 4.18a5.985 5.985 0 0 0-3.998 2.9 6.046 6.046 0 0 0 .743 7.097 5.98 5.98 0 0 0 .51 4.911 6.051 6.051 0 0 0 6.515 2.9A5.985 5.985 0 0 0 13.26 24a6.056 6.056 0 0 0 5.772-4.206 5.99 5.99 0 0 0 3.997-2.9 6.056 6.056 0 0 0-.747-7.073zM13.26 22.43a4.476 4.476 0 0 1-2.876-1.04l.141-.081 4.779-2.758a.795.795 0 0 0 .392-.681v-6.737l2.02 1.168a.071.071 0 0 1 .038.052v5.583a4.504 4.504 0 0 1-4.494 4.494zM3.6 18.304a4.47 4.47 0 0 1-.535-3.014l.142.085 4.783 2.759a.771.771 0 0 0 .78 0l5.843-3.369v2.332a.08.08 0 0 1-.033.062L9.74 19.95a4.5 4.5 0 0 1-6.14-1.646zM2.34 7.896a4.485 4.485 0 0 1 2.366-1.973V11.6a.766.766 0 0 0 .388.676l5.815 3.355-2.02 1.168a.076.076 0 0 1-.071 0l-4.83-2.786A4.504 4.504 0 0 1 2.34 7.872zm16.597 3.855l-5.833-3.387L15.119 7.2a.076.076 0 0 1 .071 0l4.83 2.791a4.494 4.494 0 0 1-.676 8.105v-5.678a.79.79 0 0 0-.395-.682zm2.01-3.023l-.141-.085-4.774-2.782a.776.776 0 0 0-.785 0L9.409 9.23V6.897a.066.066 0 0 1 .028-.061l4.83-2.787a4.5 4.5 0 0 1 6.68 4.66zm-12.64 4.135l-2.02-1.164a.08.08 0 0 1-.038-.057V6.075a4.5 4.5 0 0 1 7.375-3.453l-.142.08L8.704 5.46a.795.795 0 0 0-.393.681zm1.097-2.365l2.602-1.5 2.607 1.5v2.999l-2.597 1.5-2.607-1.5z" />
  </svg>
);

/**
 * Waiting modal during ChatGPT OAuth. Manual redirect paste is hidden behind
 * "Having problems?" for remote deploys where localhost:1455 never hits the server.
 */
export const ChatGPTOAuthWaitingDialog: React.FC = () => {
  const {
    chatgptOAuthPending,
    submitChatGPTCallbackUrl,
    cancelChatGPTOAuth,
  } = useAuth();
  const [helpOpen, setHelpOpen] = useState(false);
  const [url, setUrl] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);

  // Reset help UI when a new OAuth attempt starts/ends.
  useEffect(() => {
    if (!chatgptOAuthPending) {
      setHelpOpen(false);
      setUrl("");
      setLocalError(null);
      setSubmitting(false);
    }
  }, [chatgptOAuthPending]);

  const handleOpenChange = (open: boolean) => {
    if (!open && chatgptOAuthPending) {
      cancelChatGPTOAuth();
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim()) return;
    setLocalError(null);
    setSubmitting(true);
    try {
      await submitChatGPTCallbackUrl(url.trim());
      setUrl("");
    } catch (err) {
      setLocalError(
        err instanceof Error ? err.message : "Failed to submit callback URL",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={chatgptOAuthPending} onOpenChange={handleOpenChange}>
      <DialogContent
        className="sm:max-w-sm"
        showCloseButton
        onPointerDownOutside={(e) => e.preventDefault()}
        onInteractOutside={(e) => e.preventDefault()}
      >
        <DialogHeader className="items-center text-center sm:text-center">
          <div className="mx-auto mb-3 flex items-center justify-center gap-3">
            <AppLogo />
            <Loader2 className="size-5 animate-spin text-muted-foreground shrink-0" />
            <ChatGPTLogo className="text-foreground" />
          </div>
          <DialogTitle>Waiting for ChatGPT</DialogTitle>
        </DialogHeader>

        <div className="pt-5 space-y-0">
          <div className="flex justify-center">
            <button
              type="button"
              onClick={() => setHelpOpen((open) => !open)}
              className="text-sm text-muted-foreground hover:text-foreground underline underline-offset-4 transition-colors"
            >
              Having problems?
            </button>
          </div>

          <div
            className={cn(
              "grid w-full transition-all !duration-200 ease-in-out",
              helpOpen
                ? "grid-rows-[1fr] opacity-100"
                : "grid-rows-[0fr] opacity-0",
            )}
          >
            <div className="overflow-hidden">
              <div className="mt-3 space-y-2.5">
                <p className="text-[11px] text-muted-foreground leading-relaxed text-center">
                  Stuck or connection refuesed?
                  try paste the redirect URL here.
                </p>
                <form onSubmit={handleSubmit} className="space-y-2">
                  <input
                    type="text"
                    value={url}
                    onChange={(e) => {
                      setUrl(e.target.value);
                      if (localError) setLocalError(null);
                    }}
                    placeholder="http://localhost:1455/auth/callback?code=…"
                    className="w-full px-3 py-2 text-xs rounded-lg border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-[0.5px] focus:ring-primary/40"
                    disabled={submitting}
                    autoComplete="off"
                    spellCheck={false}
                  />
                  <button
                    type="submit"
                    disabled={submitting || !url.trim()}
                    className="w-full px-3 py-1.5 text-xs rounded-lg border border-input bg-background hover:bg-muted/60 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {submitting ? "Verifying..." : "Submit"}
                  </button>
                </form>
                {localError && (
                  <p className="text-[11px] text-destructive text-center font-medium">
                    {localError}
                  </p>
                )}
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
};
