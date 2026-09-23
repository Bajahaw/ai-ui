import { useEffect, useState } from "react";
import { RefreshCwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { applyAppUpdate, subscribeAppUpdate } from "@/lib/appUpdate";

const itemClass =
  "h-auto rounded-lg p-1.5 text-base hover:bg-muted-foreground/10 hover:text-foreground dark:hover:bg-muted-foreground/10";

export function UpdateBanner() {
  const [available, setAvailable] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const [applying, setApplying] = useState(false);

  useEffect(() => subscribeAppUpdate(setAvailable), []);

  if (!available || dismissed) return null;

  return (
    <div className="pointer-events-none fixed inset-x-0 top-4 z-[60] flex justify-center px-4">
      <div className="pointer-events-auto flex items-center gap-1 rounded-xl border border-border bg-secondary p-1 text-popover-foreground shadow-xl animate-in fade-in-0 slide-in-from-top-2">
        <span className="px-2 text-base">New version available</span>
        <Button
          variant="ghost"
          className={itemClass}
          disabled={applying}
          onClick={() => {
            setApplying(true);
            applyAppUpdate();
          }}
        >
          <RefreshCwIcon
            className={applying ? "size-4 animate-spin" : "size-4"}
          />
          {applying ? "Reloading" : "Reload"}
        </Button>
        <Button
          variant="ghost"
          className={`${itemClass} text-muted-foreground`}
          onClick={() => setDismissed(true)}
        >
          Later
        </Button>
      </div>
    </div>
  );
}
