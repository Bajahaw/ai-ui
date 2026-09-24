import { useState } from "react";
import { Check, Moon, Palette, Sun } from "lucide-react";
import { Label } from "@/components/ui/label";
import { useTheme } from "@/components/theme-provider";
import { useSettingsData } from "@/hooks/useSettingsData";
import { ACCENTS, ACCENT_SETTING_KEY, type AccentId } from "@/lib/accent";
import { cn } from "@/lib/utils";

const THEMES = [
  { id: "light", label: "Light", icon: Sun },
  { id: "dark", label: "Dark", icon: Moon },
] as const;

export const AppearanceSection = () => {
  const { theme, setTheme, accent, setAccent } = useTheme();
  const { updateSettingsLocal, saveSettings } = useSettingsData();
  const [saveError, setSaveError] = useState<string | null>(null);

  const handleAccent = async (next: AccentId) => {
    if (next === accent) return;
    const previous = accent;
    setAccent(next);
    setSaveError(null);
    try {
      updateSettingsLocal(ACCENT_SETTING_KEY, next);
      await saveSettings();
    } catch {
      setAccent(previous);
      setSaveError("Couldn't save the accent colour. Check your connection and try again.");
    }
  };

  return (
    <div className="space-y-8 max-w-2xl">
      <h3 className="text-lg font-medium flex items-center gap-2">
        <Palette className="h-5 w-5" />
        Appearance
      </h3>

      <div className="space-y-6">
        <div className="flex justify-between items-center border-b border-border pb-4">
          <div>
            <Label className="!mb-0">Theme</Label>
            <p className="text-xs text-muted-foreground mt-1">
              Saved on this device.
            </p>
          </div>
          <div
            role="radiogroup"
            aria-label="Theme"
            className="inline-flex rounded-lg border border-border bg-secondary/50 p-0.5"
          >
            {THEMES.map(({ id, label, icon: Icon }) => (
              <button
                key={id}
                type="button"
                role="radio"
                aria-checked={theme === id}
                onClick={() => setTheme(id)}
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
                  theme === id
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                <Icon className="h-4 w-4" />
                {label}
              </button>
            ))}
          </div>
        </div>

        <div className="space-y-3">
          <div>
            <Label className="!mb-0">Accent colour</Label>
            <p className="text-xs text-muted-foreground mt-1">
              Used for buttons, links, switches, focus rings and text
              selection. Synced to your account.
            </p>
          </div>
          <div
            role="radiogroup"
            aria-label="Accent colour"
            className="grid grid-cols-4 sm:grid-cols-7 gap-3 pt-1"
          >
            {ACCENTS.map(({ id, label }) => {
              const selected = accent === id;
              return (
                <button
                  key={id}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  aria-label={label}
                  data-accent={id}
                  onClick={() => handleAccent(id)}
                  className="group flex flex-col items-center gap-2 rounded-lg py-2 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                >
                  <span
                    className={cn(
                      "flex h-9 w-9 items-center justify-center rounded-full bg-primary text-primary-foreground transition-transform group-hover:scale-105",
                      selected
                        ? "ring-2 ring-primary ring-offset-2 ring-offset-background"
                        : "ring-1 ring-border/60",
                    )}
                  >
                    {selected && <Check className="h-4 w-4" strokeWidth={3} />}
                  </span>
                  <span
                    className={cn(
                      "text-xs",
                      selected
                        ? "text-foreground font-medium"
                        : "text-muted-foreground",
                    )}
                  >
                    {label}
                  </span>
                </button>
              );
            })}
          </div>
          {saveError && (
            <p className="text-xs text-destructive" role="alert">
              {saveError}
            </p>
          )}
        </div>
      </div>
    </div>
  );
};
