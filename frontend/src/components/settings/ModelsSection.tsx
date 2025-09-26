import React, { useCallback, useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";

import { Input } from "@/components/ui/input";

import { Card } from "@/components/ui/card";

import { Badge } from "@/components/ui/badge";

import {
  RefreshCw,
  CheckSquare,
  Square,
  Eye,
  EyeOff,
  Search,
  ShieldCheck,
} from "lucide-react";

import {
  getAllModels,
  updateModelEnableFlags,
  locallyApplyEnableFlags,
} from "@/lib/api/models";

import { Model } from "@/lib/api/types";

import { useProviders } from "@/hooks/useProviders";

/**

 * ModelsSection (refined styles)

 *

 * - Uniform control heights
 * - Minimal toggle (On/Off pill) replacing switch
 * - No transient spinners for quick actions
 * - Scroll confined only to models list (section header static)
 * - Transparent checkboxes (no colored background)
 */

export const ModelsSection: React.FC = () => {
  const { syncModels } = useProviders();

  const [models, setModels] = useState<Model[]>([]);

  const [loading, setLoading] = useState<boolean>(true);

  const [error, setError] = useState<string | null>(null);

  // UI state

  const [search, setSearch] = useState("");

  const [selection, setSelection] = useState<Set<string>>(new Set());

  const [pendingIds, setPendingIds] = useState<Set<string>>(new Set());

  const [bulkBusy, setBulkBusy] = useState(false);
  const fetchModels = useCallback(async (): Promise<Model[]> => {
    setLoading(true);

    setError(null);

    try {
      const res = await getAllModels();

      const normalized = res.models.map((m) => ({
        ...m,

        is_enabled: m.is_enabled !== false,
      }));

      setModels(normalized);

      return normalized;
    } catch (e: any) {
      setError(e?.message || "Failed to load models");

      return [];
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchModels().then((latest) => syncModels(latest));
  }, [fetchModels, syncModels]);

  const filtered = useMemo(() => {
    if (!search.trim()) return models;

    const q = search.toLowerCase();

    return models.filter(
      (m) =>
        m.name.toLowerCase().includes(q) ||
        m.provider.toLowerCase().includes(q),
    );
  }, [models, search]);

  const enabledInFiltered = filtered.filter((m) => m.is_enabled).length;

  const allVisibleSelected =
    filtered.length > 0 && filtered.every((m) => selection.has(m.id));

  const anySelected = selection.size > 0;

  const toggleSelect = (id: string) => {
    setSelection((prev) => {
      const next = new Set(prev);

      if (next.has(id)) next.delete(id);
      else next.add(id);

      return next;
    });
  };

  const selectAllVisible = () =>
    setSelection(new Set(filtered.map((m) => m.id)));

  const clearSelection = () => setSelection(new Set());

  const applyFlagUpdatesOptimistic = (
    updates: { id: string; is_enabled: boolean }[],
  ) => {
    if (!updates.length) return;

    setModels((prev) => locallyApplyEnableFlags(prev, updates));
  };

  const commitFlagUpdates = async (ids: string[], enabled: boolean) => {
    if (ids.length === 0) return;

    applyFlagUpdatesOptimistic(ids.map((id) => ({ id, is_enabled: enabled })));

    setPendingIds((prev) => {
      const next = new Set(prev);

      ids.forEach((id) => next.add(id));

      return next;
    });

    try {
      await updateModelEnableFlags(
        ids.map((id) => ({ id, is_enabled: enabled })),
      );

      const latest = await fetchModels();

      syncModels(latest);
    } catch {
      const latest = await fetchModels();

      syncModels(latest);
    } finally {
      setPendingIds((prev) => {
        const next = new Set(prev);

        ids.forEach((id) => next.delete(id));

        return next;
      });
    }
  };

  const handleSingleToggle = (model: Model) => {
    void commitFlagUpdates([model.id], !model.is_enabled);
  };

  const handleBulk = async (enable: boolean) => {
    if (!anySelected) return;

    setBulkBusy(true);

    await commitFlagUpdates(Array.from(selection), enable);

    setBulkBusy(false);

    clearSelection();
  };

  const enabledSelectedCount = Array.from(selection).filter(
    (id) => models.find((m) => m.id === id)?.is_enabled,
  ).length;

  const disabledSelectedCount = selection.size - enabledSelectedCount;

  return (
    <div className="flex flex-col h-full gap-5">
      {/* Header & Actions (non-scrollable) */}
      <div className="flex-shrink-0 flex flex-col gap-4">
        <div className="flex items-center justify-between flex-wrap gap-3">
          <h3 className="text-lg font-medium flex items-center gap-2">
            <ShieldCheck className="h-5 w-5" />
            Models
          </h3>

          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => fetchModels()}
              disabled={bulkBusy || loading}
              title="Refresh models"
              className="h-8"
            >
              <RefreshCw className="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div className="flex flex-col sm:flex-row gap-3">
          <div className="relative flex-1">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />

            <Input
              placeholder="Search models or providers..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-8 h-8 text-sm focus-visible:ring-0 focus-visible:outline-none focus-visible:border-border border-border/70"
              disabled={loading}
            />
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            <Button
              size="sm"
              variant="outline"
              onClick={() =>
                allVisibleSelected ? clearSelection() : selectAllVisible()
              }
              disabled={filtered.length === 0 || loading}
              className="gap-1 h-8"
            >
              {allVisibleSelected ? (
                <>
                  <CheckSquare className="h-4 w-4" />
                  Deselect
                </>
              ) : (
                <>
                  <Square className="h-4 w-4" />
                  Select All
                </>
              )}
            </Button>

            <Button
              size="sm"
              onClick={() => handleBulk(true)}
              disabled={!anySelected || bulkBusy || loading}
              className="gap-1 h-8"
            >
              <Eye className="h-4 w-4" />
              Enable
            </Button>

            <Button
              size="sm"
              variant="outline"
              onClick={() => handleBulk(false)}
              disabled={!anySelected || bulkBusy || loading}
              className="gap-1 h-8"
            >
              <EyeOff className="h-4 w-4" />
              Disable
            </Button>
          </div>
        </div>

        <div className="text-xs text-muted-foreground flex flex-wrap gap-x-4 gap-y-1">
          <span>
            {filtered.length} model{filtered.length !== 1 && "s"} shown
          </span>

          <span>{enabledInFiltered} enabled in view</span>

          {selection.size > 0 && (
            <span>
              Selected: {selection.size} (E:{enabledSelectedCount} / D:
              {disabledSelectedCount})
            </span>
          )}
        </div>

        {error && (
          <div className="text-xs text-red-600 dark:text-red-400">{error}</div>
        )}
      </div>

      {/* Scrollable Content */}
      <div className="flex-1 min-h-0">
        {loading && (
          <Card className="p-8 text-center bg-transparent border-dashed h-full">
            <div className="text-sm text-muted-foreground">
              Loading models...
            </div>
          </Card>
        )}

        {!loading && filtered.length === 0 && (
          <Card className="p-8 text-center bg-transparent border-dashed h-full">
            <p className="text-sm text-muted-foreground">
              {search ? "No models match your search." : "No models available."}
            </p>
          </Card>
        )}

        {!loading && filtered.length > 0 && (
          <div className="space-y-2 h-full overflow-y-auto pr-1">
            {filtered.map((model) => {
              const selected = selection.has(model.id);

              const pending = pendingIds.has(model.id);

              return (
                <div
                  key={model.id}
                  className={`flex items-center gap-3 rounded-lg border px-3 py-2 text-sm transition-colors ${
                    selected
                      ? "bg-primary/5 border-primary/40"
                      : "border-border/50 hover:bg-muted/40"
                  }`}
                >
                  <input
                    type="checkbox"
                    checked={selected}
                    onChange={() => toggleSelect(model.id)}
                    className="h-4 w-4 cursor-pointer rounded-md bg-transparent border border-border checked:bg-primary checked:border-primary transition-colors"
                  />

                  <div className="flex-1 min-h-0 overflow-hidden">
                    <div className="flex items-center gap-2">
                      <span
                        className="font-medium truncate max-w-[220px]"
                        title={model.name}
                      >
                        {model.name}
                      </span>

                      <Badge
                        variant="outline"
                        className="text-[10px] tracking-wide"
                        title={`Provider: ${model.provider}`}
                      >
                        {model.provider}
                      </Badge>
                    </div>

                    <div className="text-[11px] text-muted-foreground truncate max-w-[260px]">
                      {model.id}
                    </div>
                  </div>

                  <button
                    onClick={() => handleSingleToggle(model)}
                    disabled={pending || bulkBusy || loading}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none shadow-sm ${
                      model.is_enabled
                        ? "bg-primary/80 hover:bg-primary text-primary-foreground"
                        : "bg-muted hover:bg-muted/70"
                    } ${pending ? "opacity-50 cursor-wait" : "cursor-pointer"}`}
                    title={model.is_enabled ? "Disable model" : "Enable model"}
                  >
                    <span
                      className={`inline-block h-5 w-5 transform rounded-full bg-background shadow transition-transform duration-200 ease-out ${
                        model.is_enabled ? "translate-x-5" : "translate-x-1"
                      }`}
                    />
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};

export default ModelsSection;
