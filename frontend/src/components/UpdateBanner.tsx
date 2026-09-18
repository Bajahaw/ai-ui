import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { applyAppUpdate, subscribeAppUpdate } from "@/lib/appUpdate";

export function UpdateBanner() {
  const [available, setAvailable] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const [applying, setApplying] = useState(false);

  useEffect(() => subscribeAppUpdate(setAvailable), []);

  if (!available || dismissed) return null;

  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex justify-center px-4">
      <div className="pointer-events-auto flex items-center gap-3 rounded-lg border bg-card px-3 py-2 text-sm shadow-lg">
        <span>New version available</span>
        <Button
          size="sm"
          disabled={applying}
          onClick={() => {
            setApplying(true);
            applyAppUpdate();
          }}
        >
          {applying ? "Reloading" : "Reload"}
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => setDismissed(true)}
        >
          Later
        </Button>
      </div>
    </div>
  );
}
