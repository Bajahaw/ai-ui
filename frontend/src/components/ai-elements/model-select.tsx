"use client";

import { Fragment, useMemo, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { defaultRangeExtractor, useVirtualizer } from "@tanstack/react-virtual";
import { Loader2, Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn, createSearchMatcher } from "@/lib/utils";
import { formatProviderSelectLabel } from "@/lib/api/providers";

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

const ModelLabel = ({ model }: { model: ModelOption }) => (
  <div className="max-w-[280px] overflow-hidden text-ellipsis text-nowrap select-none translate-y-[-2px]">
    <span className="font-medium">{model.name}</span>{" "}
    <span className="text-xs text-muted-foreground">
      {formatProviderSelectLabel(model.provider)}
    </span>
  </div>
);

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
  const [uncontrolledValue, setUncontrolledValue] = useState<string>();
  const selectValue = value === undefined
    ? uncontrolledValue
    : typeof value === "string" && value.length > 0 ? value : undefined;
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [focusedIndex, setFocusedIndex] = useState(-1);
  const searchRef = useRef<HTMLInputElement>(null);
  const viewportRef = useRef<HTMLDivElement>(null);
  const searchFocusRef = useRef(false);
  const typeaheadRef = useRef({ text: "", time: 0 });
  const selectedModel = models.find((model) => model.id === selectValue);
  const filteredModels = useMemo(() => {
    const matches = createSearchMatcher(search);
    return models.filter((model) =>
      matches(model.name, model.provider, formatProviderSelectLabel(model.provider)),
    );
  }, [models, search]);
  const selectedIndex = filteredModels.findIndex((model) => model.id === selectValue);
  const sizingModel = useMemo(() => filteredModels.reduce<ModelOption | undefined>(
    (longest, model) => {
      const length = model.name.length + formatProviderSelectLabel(model.provider).length;
      const longestLength = longest
        ? longest.name.length + formatProviderSelectLabel(longest.provider).length
        : -1;
      return length > longestLength ? model : longest;
    },
    undefined,
  ), [filteredModels]);

  const rowVirtualizer = useVirtualizer({
    count: loading ? 0 : filteredModels.length,
    enabled: open,
    getScrollElement: () => viewportRef.current,
    estimateSize: () => 40,
    getItemKey: (index) => filteredModels[index].id,
    overscan: 5,
    scrollMargin: 40,
    scrollPaddingStart: 40,
    initialRect: { width: 240, height: 288 },
    rangeExtractor: (range) => Array.from(new Set([
      ...defaultRangeExtractor(range),
      0,
      filteredModels.length - 1,
      selectedIndex,
      focusedIndex,
    ])).filter((index) => index >= 0 && index < filteredModels.length).sort((a, b) => a - b),
  });
  const virtualRows = rowVirtualizer.getVirtualItems();

  const focusModel = (index: number) => {
    if (!filteredModels[index]) return;
    searchFocusRef.current = false;
    flushSync(() => setFocusedIndex(index));
    rowVirtualizer.scrollToIndex(index, { align: "auto" });
    viewportRef.current?.querySelector<HTMLElement>(`[data-model-index="${index}"]`)
      ?.focus({ preventScroll: true });
  };

  const computedPlaceholder =
    placeholder ??
    (loading
      ? "Loading models..."
      : models.length === 0
        ? emptyMessage
        : "Select a model");

  const variantStyles: Record<
    NonNullable<ModelSelectProps["variant"]>,
    string
  > = {
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
    <Select
      value={selectValue ?? ""}
      onValueChange={(nextValue) => {
        setUncontrolledValue(nextValue);
        onChange?.(nextValue);
      }}
      disabled={isDisabled}
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen);
        setSearch("");
        setFocusedIndex(-1);
        searchFocusRef.current = false;
        typeaheadRef.current = { text: "", time: 0 };
      }}
    >
      <SelectTrigger
        id={triggerId}
        aria-label={triggerAriaLabel}
        size={size}
        className={cn(
          "flex w-fit items-center justify-between gap-2 rounded-lg !border-none !bg-transparent hover:!bg-secondary/80 transition-all !duration-100 data-[placeholder]:!text-muted-foreground focus:ring-0 focus:ring-offset-0 focus-visible:ring-0 focus-visible:ring-offset-0",
          variantStyles[variant],
          sizeStyles[size],
          isDisabled && "opacity-60",
          triggerClassName,
        )}
      >
        <SelectValue placeholder={computedPlaceholder}>
          {selectedModel ? <ModelLabel model={selectedModel} /> : undefined}
        </SelectValue>
      </SelectTrigger>

      <SelectContent
        viewportRef={viewportRef}
        onFocusCapture={(event) => {
          if (searchFocusRef.current && event.target !== searchRef.current) {
            event.preventDefault();
            searchRef.current?.focus({ preventScroll: true });
          }
        }}
        onPointerDownCapture={(event) => {
          if (event.target instanceof Element && event.target.closest('[role="option"]')) {
            searchFocusRef.current = false;
          }
        }}
        onKeyDownCapture={(event) => {
          if (event.target === searchRef.current || event.nativeEvent.isComposing) return;
          const target = event.target instanceof Element
            ? event.target.closest<HTMLElement>("[data-model-index]")
            : null;
          const current = target ? Number(target.dataset.modelIndex) : Math.max(selectedIndex, 0);
          const pageSize = Math.max(1, Math.floor(((viewportRef.current?.clientHeight || 288) - 40) / 40));
          const destinations: Record<string, number> = {
            ArrowDown: current + 1,
            ArrowUp: current - 1,
            Home: 0,
            End: filteredModels.length - 1,
            PageDown: current + pageSize,
            PageUp: current - pageSize,
          };
          const destination = destinations[event.key];
          if (destination !== undefined && filteredModels.length > 0) {
            event.preventDefault();
            event.stopPropagation();
            focusModel(Math.max(0, Math.min(filteredModels.length - 1, destination)));
          } else if (!event.ctrlKey && !event.altKey && !event.metaKey && event.key.length === 1) {
            const now = Date.now();
            if (event.key === " " && now - typeaheadRef.current.time > 1000) return;
            const text = (now - typeaheadRef.current.time > 1000 ? "" : typeaheadRef.current.text)
              + event.key.toLowerCase();
            typeaheadRef.current = { text, time: now };
            const query = Array.from(text).every((character) => character === text[0]) ? text[0] : text;
            event.preventDefault();
            event.stopPropagation();
            for (let offset = 1; offset <= filteredModels.length; offset++) {
              const index = (current + offset) % filteredModels.length;
              if (filteredModels[index].name.toLowerCase().startsWith(query)) {
                focusModel(index);
                break;
              }
            }
          }
        }}
        className={cn(
          "max-h-72 min-w-[240px] overflow-auto rounded-xl border border-border/70 p-1 shadow-xl",
          contentClassName,
        )}
      >
        <div className="sticky top-0 z-10 bg-secondary px-3 py-2.5">
          <div className="relative h-5 translate-y-[-2px]">
            <Search
              aria-hidden="true"
              className="pointer-events-none absolute left-2 top-1/2 z-10 size-4 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              ref={searchRef}
              type="search"
              aria-label="Search models"
              placeholder="Search models or providers..."
              autoComplete="off"
              spellCheck={false}
              value={search}
              onFocus={() => {
                searchFocusRef.current = true;
              }}
              onChange={(event) => {
                setSearch(event.target.value);
                setFocusedIndex(-1);
                rowVirtualizer.scrollToOffset(0);
              }}
              onKeyDown={(event) => {
                if (event.key === "Escape") return;
                event.stopPropagation();
                if (event.nativeEvent.isComposing) return;
                if (event.key === "Enter") event.preventDefault();
                if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                  event.preventDefault();
                  focusModel(event.key === "ArrowUp" ? filteredModels.length - 1 : 0);
                }
              }}
              className="absolute inset-x-0 top-1/2 h-9 -translate-y-1/2 pl-8 pr-2 py-0 text-sm border-0 border-b rounded-none shadow-none focus-visible:ring-0 focus:border-primary focus-visible:border-primary !bg-transparent"
            />
          </div>
        </div>
        {loading ? (
          <div className="flex items-center justify-center gap-2 px-3 py-6 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" />
            Loading models...
          </div>
        ) : models.length === 0 ? (
          <div className="px-3 py-6 text-center text-sm text-muted-foreground">
            {emptyMessage}
            {helperMessage && (
              <div className="mt-1 text-xs text-muted-foreground/80">
                {helperMessage}
              </div>
            )}
          </div>
        ) : filteredModels.length === 0 ? (
          <div className="px-3 py-6 text-center text-sm text-muted-foreground">
            No models match your search.
          </div>
        ) : (
          <>
            <div aria-hidden="true" className="h-0 overflow-hidden pointer-events-none">
              {sizingModel && (
                <div className="flex items-center px-3 text-sm leading-5">
                  <ModelLabel model={sizingModel} />
                </div>
              )}
              {selectedModel && selectedIndex >= 0 && (
                <div className="flex items-center px-3 text-sm leading-5">
                  <ModelLabel model={selectedModel} />
                  <span className="size-4 shrink-0" />
                </div>
              )}
            </div>
            <div data-slot="model-options">
              {virtualRows.map((virtualRow, position) => {
                const modelOption = filteredModels[virtualRow.index];
                const previousEnd = position === 0 ? 40 : virtualRows[position - 1].end;
                return (
                  <Fragment key={modelOption.id}>
                    {virtualRow.start > previousEnd && (
                      <div aria-hidden="true" style={{ height: virtualRow.start - previousEnd }} />
                    )}
                    <SelectItem
                      value={modelOption.id}
                      data-model-index={virtualRow.index}
                      aria-posinset={virtualRow.index + 1}
                      aria-setsize={filteredModels.length}
                      onFocus={() => {
                        if (!searchFocusRef.current) setFocusedIndex(virtualRow.index);
                      }}
                      onPointerMove={(event) => {
                        if (searchFocusRef.current) event.preventDefault();
                      }}
                      className="rounded-lg px-3 py-2.5 text-sm leading-5 focus:bg-muted-foreground/10"
                    >
                      <ModelLabel model={modelOption} />
                    </SelectItem>
                  </Fragment>
                );
              })}
              <div
                aria-hidden="true"
                style={{ height: Math.max(0, rowVirtualizer.getTotalSize() - ((virtualRows.at(-1)?.end ?? 40) - 40)) }}
              />
            </div>
            {showCount && (
              <div className="border-t border-border/50 px-3 py-2 text-center text-xs text-muted-foreground">
                {filteredModels.length} model{filteredModels.length === 1 ? "" : "s"} available
              </div>
            )}
          </>
        )}
      </SelectContent>
    </Select>
  );
};
