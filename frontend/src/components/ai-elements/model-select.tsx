"use client";

import {Loader2} from "lucide-react";
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select";
import {cn} from "@/lib/utils";

export interface ModelOption {
    id: string;
    name: string;
    provider: string;
}

export interface ModelSelectProps {
    models: ModelOption[];
    value?: string | null;
    onChange?: (value: string) => void;
    placeholder?: string;
    emptyMessage?: string;
    helperMessage?: string;
    loading?: boolean;
    disabled?: boolean;
    size?: "sm" | "default";
    variant?: "ghost" | "solid";
    triggerId?: string;
    triggerAriaLabel?: string;
    triggerClassName?: string;
    contentClassName?: string;
    showCount?: boolean;
}

export const ModelSelect = ({
                                models,
                                value,
                                onChange,
                                placeholder,
                                emptyMessage = "No models available",
                                helperMessage,
                                loading = false,
                                disabled = false,
                                size = "default",
                                variant = "solid",
                                triggerId,
                                triggerAriaLabel,
                                triggerClassName,
                                contentClassName,
                                showCount = false,
                            }: ModelSelectProps) => {
    const selectValue = typeof value === "string" && value.length > 0 ? value : undefined;

    const computedPlaceholder =
        placeholder ??
        (loading
            ? "Loading models..."
            : models.length === 0
                ? emptyMessage
                : "Select a model");

    const variantStyles: Record<NonNullable<ModelSelectProps["variant"]>, string> = {
        solid:
            "border-border/70 bg-background/70 text-foreground shadow-xs hover:bg-background/80",
        ghost:
            "border-transparent bg-transparent text-foreground/80 shadow-none hover:bg-accent/40 hover:text-foreground",
    };

    const sizeStyles: Record<NonNullable<ModelSelectProps["size"]>, string> = {
        default: "px-3 text-sm",
        sm: "px-2 text-sm",
    };

    const isDisabled = disabled;

    return (
        <Select value={selectValue} onValueChange={onChange} disabled={isDisabled}>
            <SelectTrigger
                id={triggerId}
                aria-label={triggerAriaLabel}
                size={size}
                className={cn(
                    "flex w-fit items-center justify-between gap-2 rounded-lg !border-none !bg-transparent transition-colors data-[placeholder]:text-muted-foreground",
                    variantStyles[variant],
                    sizeStyles[size],
                    isDisabled && "opacity-60",
                    triggerClassName,
                )}
            >
                <SelectValue placeholder={computedPlaceholder}/>
            </SelectTrigger>

            <SelectContent
                className={cn(
                    "max-h-72 min-w-[240px] overflow-auto rounded-xl border border-border/70 p-1 shadow-xl",
                    contentClassName,
                )}
            >
                {loading ? (
                    <div className="flex items-center justify-center gap-2 px-3 py-6 text-sm text-muted-foreground">
                        <Loader2 className="size-4 animate-spin"/>
                        Loading models...
                    </div>
                ) : models.length === 0 ? (
                    <div className="px-3 py-6 text-center text-sm text-muted-foreground">
                        {emptyMessage}
                        {helperMessage && (
                            <div className="mt-1 text-xs text-muted-foreground/80">{helperMessage}</div>
                        )}
                    </div>
                ) : (
                    <>
                        {models.map((modelOption) => (
                            <SelectItem
                                key={modelOption.id}
                                value={modelOption.id}
                                className="rounded-lg px-3 py-2.5 text-sm leading-5 focus:bg-muted-foreground/10"
                            >
                                <div className="max-w-[280px] overflow-hidden text-ellipsis text-nowrap select-none">
                                    <span className="font-medium">{modelOption.name}</span> {" "}
                                    <span className="text-xs text-muted-foreground">{modelOption.provider}</span>
                                </div>
                            </SelectItem>
                        ))}

                        {showCount && (
                            <div
                                className="border-t border-border/50 px-3 py-2 text-center text-xs text-muted-foreground">
                                {models.length} model{models.length === 1 ? "" : "s"} available
                            </div>
                        )}
                    </>
                )}
            </SelectContent>
        </Select>
    );
};
