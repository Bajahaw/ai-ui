"use client";

import { cn } from "@/lib/utils";
import { ComponentProps } from "react";
import { WelcomeStats } from "@/lib/api/types";
import type { OnboardingStep } from "@/lib/onboarding";
import { MCPPresetPicker, ProviderPresetPicker } from "@/components/onboarding";

function formatStatNumber(n: number): string {
  if (n >= 1_000_000)
    return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, "")}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1).replace(/\.0$/, "")}K`;
  return n.toString();
}

const STAT_DEFS: { key: keyof WelcomeStats; label: string }[] = [
  { key: "totalTokens", label: "Tokens Out" },
  { key: "totalInputTokens", label: "Tokens In" },
  { key: "totalConversations", label: "Conversations" },
  { key: "totalMessages", label: "Messages" },
];

function StatCell({
  value,
  label,
  index,
}: {
  value: number | undefined;
  label: string;
  index: number;
}) {
  // On small screens (2-col grid): border only on right column (items 1, 3)
  // On sm+ (4-col single row): border on every item except the first (items 1, 2, 3)
  const borderClass =
    index === 0
      ? ""
      : index % 2 !== 0
        ? "border-l border-border/40" // items 1 & 3: always
        : "sm:border-l sm:border-border/40"; // item 2: sm+ only
  return (
    <div
      className={cn("flex flex-col items-center gap-2 py-5 px-4", borderClass)}
    >
      <span
        className={cn(
          "text-3xl sm:text-4xl font-thin tabular-nums tracking-tight text-foreground leading-none whitespace-nowrap",
          value === undefined ? "invisible" : "animate-fade-in-slow",
        )}
      >
        {value === undefined ? "\u00A0" : formatStatNumber(value)}
      </span>
      <span
        className={cn(
          "text-[9px] font-normal tracking-[0.16em] uppercase text-muted-foreground text-center whitespace-nowrap",
          value === undefined ? "invisible" : "animate-fade-in-slow",
        )}
      >
        {label}
      </span>
    </div>
  );
}

const ONBOARDING_MESSAGE: Record<NonNullable<OnboardingStep>, string> = {
  provider: "Connect a provider to start chatting",
  mcp: "Give it tools with an MCP server, or just start chatting",
};

export interface WelcomeProps extends ComponentProps<"div"> {
  stats?: WelcomeStats;
  isLoading?: boolean;
  message?: string;
  /** When set, replaces the stats grid with a setup step. */
  onboardingStep?: OnboardingStep;
  onSkipOnboarding?: () => void;
}

export const Welcome = ({
  className,
  stats,
  isLoading = false,
  message,
  onboardingStep = null,
  onSkipOnboarding,
  ...props
}: WelcomeProps) => {
  const displayMessage = onboardingStep
    ? ONBOARDING_MESSAGE[onboardingStep]
    : message;
  const readyStats = isLoading ? undefined : stats;

  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center w-full px-4 select-none animate-fade-in",
        className,
      )}
      {...props}
    >
      <h1 className="text-3xl sm:text-4xl font-semibold text-foreground tracking-tight mb-8 text-center">
        How can I help you?
      </h1>

      <p
        className={cn(
          "mb-6 max-w-md text-center text-sm text-muted-foreground",
          !displayMessage && "invisible",
        )}
      >
        {displayMessage ?? "\u00A0"}
      </p>

      <div className="w-full max-w-lg">
        <div className="border-t border-border/40" />

        {onboardingStep === "provider" ? (
          <ProviderPresetPicker className="py-1" />
        ) : onboardingStep === "mcp" ? (
          <MCPPresetPicker className="py-1" />
        ) : (
          /* 2×2 on mobile, 4-col row on sm+ */
          <div className="grid grid-cols-2 sm:grid-cols-4">
            {STAT_DEFS.map(({ key, label }, i) => (
              <StatCell
                key={key}
                value={readyStats?.[key]}
                label={label}
                index={i}
              />
            ))}
          </div>
        )}

        <div className="border-t border-border/40" />

        {onboardingStep === "mcp" && onSkipOnboarding && (
          <div className="flex justify-center pt-3">
            <button
              type="button"
              onClick={onSkipOnboarding}
              className="text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Skip for now
            </button>
          </div>
        )}
      </div>
    </div>
  );
};
