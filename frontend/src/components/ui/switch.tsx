import * as React from "react";
import { cn } from "@/lib/utils";

interface SwitchProps {
  checked: boolean;
  onCheckedChange: () => void;
  disabled?: boolean;
  title?: string;
  className?: string;
}

export const Switch = React.forwardRef<HTMLButtonElement, SwitchProps>(
  ({ checked, onCheckedChange, disabled, title, className }, ref) => {
    return (
      <button
        ref={ref}
        onClick={onCheckedChange}
        disabled={disabled}
        className={cn(
          "relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none shadow-sm",
          checked
            ? "bg-primary/80 hover:bg-primary text-primary-foreground"
            : "bg-muted hover:bg-muted/70",
          disabled ? "opacity-50 cursor-wait" : "cursor-pointer",
          className
        )}
        title={title}
        type="button"
        role="switch"
        aria-checked={checked}
      >
        <span
          className={cn(
            "inline-block h-5 w-5 transform rounded-full bg-background shadow transition-transform duration-200 ease-out",
            checked ? "translate-x-5" : "translate-x-1"
          )}
        />
      </button>
    );
  }
);

Switch.displayName = "Switch";
