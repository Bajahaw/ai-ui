"use client";

import { cn } from "@/lib/utils";
import { ComponentProps, useEffect, useRef, useState } from "react";
import { WelcomeStats } from "@/lib/api/types";

// Cubic ease-in-out — derivative is 0 at both ends so it genuinely crawls
// in/out, and the curve never plateaus early like a sigmoid does.
function cubicEaseInOut(t: number): number {
  return t < 0.5
    ? 4 * t * t * t
    : 1 - Math.pow(-2 * t + 2, 3) / 2;
}

function useCountUp(target: number, duration = 3000): number {
  const [value, setValue] = useState(0);
  const rafRef = useRef<number | null>(null);
  const startRef = useRef<number | null>(null);

  useEffect(() => {
    if (target === 0) { setValue(0); return; }
    setValue(0);
    startRef.current = null;

    const timeout = setTimeout(() => {
      const step = (ts: number) => {
        if (startRef.current === null) startRef.current = ts;
        const progress = Math.min((ts - startRef.current) / duration, 1);
        setValue(Math.round(cubicEaseInOut(progress) * target));
        if (progress < 1) rafRef.current = requestAnimationFrame(step);
      };
      rafRef.current = requestAnimationFrame(step);
    }, 0);

    return () => {
      clearTimeout(timeout);
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
    };
  }, [target, duration]);

  return value;
}

function formatStatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, "")}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1).replace(/\.0$/, "")}K`;
  return n.toString();
}

const STAT_DEFS: { key: keyof WelcomeStats; label: string }[] = [
  { key: "totalTokens",        label: "Tokens Out"    },
  { key: "totalInputTokens",   label: "Tokens In"     },
  { key: "totalConversations", label: "Conversations" },
  { key: "totalMessages",      label: "Messages"      },
];

function StatCell({ value, label, index }: {
  value: number;
  label: string;
  index: number;
}) {
  const animated = useCountUp(value, 3000);
  // On small screens (2-col grid): border only on right column (items 1, 3)
  // On sm+ (4-col single row): border on every item except the first (items 1, 2, 3)
  const borderClass =
    index === 0 ? "" :
    index % 2 !== 0 ? "border-l border-border/40" :   // items 1 & 3: always
    "sm:border-l sm:border-border/40";                 // item 2: sm+ only
  return (
    <div className={cn(
      "flex flex-col items-center gap-2 py-5 px-4",
      borderClass,
    )}>
      <span className="text-3xl sm:text-4xl font-thin tabular-nums tracking-tight text-foreground leading-none whitespace-nowrap">
        {formatStatNumber(animated)}
      </span>
      <span className="text-[9px] font-normal tracking-[0.16em] uppercase text-muted-foreground text-center whitespace-nowrap">
        {label}
      </span>
    </div>
  );
}

export interface WelcomeProps extends ComponentProps<"div"> {
  stats?: WelcomeStats;
}

export const Welcome = ({ className, stats, ...props }: WelcomeProps) => {
  const allZero = !stats || (
    stats.totalTokens === 0 &&
    stats.totalInputTokens === 0 &&
    stats.totalConversations === 0 &&
    stats.totalMessages === 0
  );

  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center w-full px-4 select-none",
        className,
      )}
      {...props}
    >
      <h1 className="text-3xl sm:text-4xl font-semibold text-foreground tracking-tight mb-8 text-center">
        How can I help you?
      </h1>

      <div className="w-full max-w-lg">
        <div className="border-t border-border/40" />

        {/* 2×2 on mobile, 4-col row on sm+ */}
        <div className="grid grid-cols-2 sm:grid-cols-4">
          {STAT_DEFS.map(({ key, label }, i) => (
            <StatCell
              key={key}
              value={stats?.[key] ?? 0}
              label={label}
              index={i}
            />
          ))}
        </div>

        <div className="border-t border-border/40" />

        {allZero && (
          <p className="mt-5 text-center text-[9px] tracking-[0.25em] uppercase text-muted-foreground/50">
            Your stats will appear here
          </p>
        )}
      </div>
    </div>
  );
};
