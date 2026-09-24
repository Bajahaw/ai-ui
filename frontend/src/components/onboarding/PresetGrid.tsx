import { Loader2, Plus } from "lucide-react";
import { cn } from "@/lib/utils";
import type { BrandIcon } from "@/components/icons/brand-icons";

interface PresetItem {
  id: string;
  name: string;
  icon: BrandIcon;
}

interface PresetGridProps<T extends PresetItem> {
  items: T[];
  onSelect: (item: T) => void;
  /** Trailing tile for a blank form; omitted when undefined. */
  onOther?: () => void;
  otherLabel?: string;
  busyId?: string | null;
  disabled?: boolean;
  className?: string;
}

const tileClass =
  "flex flex-col items-center justify-center gap-2 py-4 px-2 rounded-lg text-foreground/80 hover:bg-muted/60 hover:text-foreground transition-colors disabled:opacity-50 disabled:pointer-events-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";
const labelClass =
  "text-[9px] font-normal tracking-[0.16em] uppercase text-muted-foreground text-center whitespace-nowrap";

export function PresetGrid<T extends PresetItem>({
  items,
  onSelect,
  onOther,
  otherLabel = "Other",
  busyId,
  disabled,
  className,
}: PresetGridProps<T>) {
  return (
    <div className={cn("grid grid-cols-4", className)}>
      {items.map((item) => {
        const Icon = item.icon;
        const busy = busyId === item.id;
        return (
          <button
            key={item.id}
            type="button"
            className={tileClass}
            onClick={() => onSelect(item)}
            disabled={disabled || busy}
            title={item.name}
          >
            {busy ? (
              <Loader2 className="size-6 animate-spin" />
            ) : (
              <Icon className="size-6" />
            )}
            <span className={labelClass}>{item.name}</span>
          </button>
        );
      })}
      {onOther && (
        <button
          type="button"
          className={tileClass}
          onClick={onOther}
          disabled={disabled}
          title={otherLabel}
        >
          <Plus className="size-6" />
          <span className={labelClass}>{otherLabel}</span>
        </button>
      )}
    </div>
  );
}
