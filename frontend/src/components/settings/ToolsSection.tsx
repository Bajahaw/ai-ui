import React, {useMemo, useState} from "react";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Card} from "@/components/ui/card";
import {CheckSquare, Eye, EyeOff, Search, ShieldAlert, Square, Wrench,} from "lucide-react";
import {locallyApplyToolFlags} from "@/lib/api/tools";
import {Tool} from "@/lib/api/types";
import {useSettingsData} from "@/hooks/useSettingsData";

/**
 * ToolsSection
 * 
 * Similar to ModelsSection but for tool management
 * - Enable/disable tools
 * - Require approval flag for tools
 * - Bulk operations
 * - Search/filter functionality
 */
export const ToolsSection: React.FC = () => {
  const { data, updateToolsLocal, saveTools } = useSettingsData();
  const [search, setSearch] = useState("");
  const [selection, setSelection] = useState<Set<string>>(new Set());
  const [pendingIds, setPendingIds] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);

  const filtered = useMemo(() => {
    if (!search.trim()) return data.tools;
    const q = search.toLowerCase();
    return data.tools.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        (t.description && t.description.toLowerCase().includes(q)) ||
        (t.mcp_server_id && t.mcp_server_id.toLowerCase().includes(q)),
    );
  }, [data.tools, search]);

  const enabledInFiltered = filtered.filter((t) => t.is_enabled).length;
  const approvalRequiredInFiltered = filtered.filter(
    (t) => t.require_approval,
  ).length;

  const allVisibleSelected =
    filtered.length > 0 && filtered.every((t) => selection.has(t.id));
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
    setSelection(new Set(filtered.map((t) => t.id)));
  const clearSelection = () => setSelection(new Set());

  const applyFlagUpdatesOptimistic = (
    updates: {
      id: string;
      is_enabled?: boolean;
      require_approval?: boolean;
    }[],
  ) => {
    if (!updates.length) return;
    const updated = locallyApplyToolFlags(data.tools, updates);
    updateToolsLocal(updated);
  };

  const commitFlagUpdates = async (
    ids: string[],
    flags: { is_enabled?: boolean; require_approval?: boolean },
  ) => {
    if (ids.length === 0) return;

    const updates = ids.map((id) => ({ id, ...flags }));
    applyFlagUpdatesOptimistic(updates);

    setPendingIds((prev) => {
      const next = new Set(prev);
      ids.forEach((id) => next.add(id));
      return next;
    });

    try {
      const updatedTools = data.tools.map((t) => {
        if (!ids.includes(t.id)) return t;
        return {
          ...t,
          ...(flags.is_enabled !== undefined && {
            is_enabled: flags.is_enabled,
          }),
          ...(flags.require_approval !== undefined && {
            require_approval: flags.require_approval,
          }),
        };
      });
      await saveTools(updatedTools);
    } finally {
      setPendingIds((prev) => {
        const next = new Set(prev);
        ids.forEach((id) => next.delete(id));
        return next;
      });
    }
  };

  const handleSingleToggleEnabled = (tool: Tool) => {
    void commitFlagUpdates([tool.id], { is_enabled: !tool.is_enabled });
  };

  const handleSingleToggleApproval = (tool: Tool) => {
    void commitFlagUpdates([tool.id], {
      require_approval: !tool.require_approval,
    });
  };

  const handleBulkEnable = async (enable: boolean) => {
    if (!anySelected) return;
    setBulkBusy(true);
    await commitFlagUpdates(Array.from(selection), { is_enabled: enable });
    setBulkBusy(false);
    clearSelection();
  };

  const enabledSelectedCount = Array.from(selection).filter(
    (id) => data.tools.find((t) => t.id === id)?.is_enabled,
  ).length;
  const disabledSelectedCount = selection.size - enabledSelectedCount;

  return (
    <div className="flex flex-col h-full gap-5">
      {/* Header & Actions (non-scrollable) */}
      <div className="flex-shrink-0 flex flex-col gap-4">
        <div className="flex items-center justify-between flex-wrap gap-3">
          <h3 className="text-lg font-medium flex items-center gap-2">
            <Wrench className="h-5 w-5" />
            Tools
          </h3>

          <div className="flex items-center gap-2">
          </div>
        </div>

        <div className="flex flex-col sm:flex-row gap-3">
          <div className="relative flex-1">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search tools..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-8 h-8 text-sm focus-visible:ring-0 focus-visible:outline-none focus-visible:border-border border-border/70"
            />
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            <Button
              size="sm"
              variant="outline"
              onClick={() =>
                allVisibleSelected ? clearSelection() : selectAllVisible()
              }
              disabled={filtered.length === 0}
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
              onClick={() => handleBulkEnable(true)}
              disabled={!anySelected || bulkBusy}
              className="gap-1 h-8"
            >
              <Eye className="h-4 w-4" />
              Enable
            </Button>

            <Button
              size="sm"
              variant="outline"
              onClick={() => handleBulkEnable(false)}
              disabled={!anySelected || bulkBusy}
              className="gap-1 h-8"
            >
              <EyeOff className="h-4 w-4" />
              Disable
            </Button>
          </div>
        </div>

        <div className="text-xs text-muted-foreground flex flex-wrap gap-x-4 gap-y-1">
          <span>
            {filtered.length} tool{filtered.length !== 1 && "s"} shown
          </span>
          <span>{enabledInFiltered} enabled</span>
          <span>{approvalRequiredInFiltered} require approval</span>
          {selection.size > 0 && (
            <span>
              Selected: {selection.size} (E:{enabledSelectedCount} / D:
              {disabledSelectedCount})
            </span>
          )}
        </div>


      </div>

      {/* Scrollable Content */}
      <div className="flex-1 min-h-0">
        {filtered.length === 0 && (
          <Card className="p-8 text-center bg-transparent border-dashed h-full">
            <p className="text-sm text-muted-foreground">
              {search ? "No tools match your search." : "No tools available."}
            </p>
          </Card>
        )}

        {filtered.length > 0 && (
          <div className="space-y-2 h-full overflow-y-auto pr-1">
            {filtered.map((tool) => {
              const selected = selection.has(tool.id);
              const pending = pendingIds.has(tool.id);

              return (
                <div
                  key={tool.id}
                  className={`flex items-center gap-3 rounded-lg border px-3 py-2 text-sm transition-colors ${
                    selected
                      ? "bg-primary/5 border-primary/40"
                      : "border-border/50 hover:bg-muted/40"
                  }`}
                >
                  <input
                    type="checkbox"
                    checked={selected}
                    onChange={() => toggleSelect(tool.id)}
                    className="h-4 w-4 cursor-pointer rounded-md bg-transparent border border-border checked:bg-primary checked:border-primary transition-colors"
                  />

                  <div className="flex-1 min-h-0 overflow-hidden">
                    <div className="flex items-center gap-2">
                      <span
                        className="font-medium truncate max-w-[200px] sm:max-w-[300px]"
                        title={tool.name}
                      >
                        {tool.name}
                      </span>
                      {tool.require_approval && (
                        <ShieldAlert className="h-3 w-3 text-amber-600 dark:text-amber-400 flex-shrink-0" />
                      )}
                    </div>
                    {tool.description && (
                      <div className="text-[11px] text-muted-foreground truncate max-w-[200px] sm:max-w-[300px]">
                        {tool.description}
                      </div>
                    )}
                    {tool.mcp_server_id && (
                      <div className="text-[10px] text-muted-foreground/70 truncate max-w-[200px] sm:max-w-[300px]">
                        MCP: {tool.mcp_server_id}
                      </div>
                    )}
                  </div>

                  <div className="flex items-center gap-2">
                    {/* Approval Toggle */}
                    <button
                      onClick={() => handleSingleToggleApproval(tool)}
                      disabled={pending || bulkBusy}
                      className={`p-1.5 rounded transition-colors ${
                        tool.require_approval
                          ? "bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400"
                          : "bg-muted hover:bg-muted/70 text-muted-foreground"
                      } ${pending ? "opacity-50 cursor-wait" : "cursor-pointer"}`}
                      title={
                        tool.require_approval
                          ? "Remove approval requirement"
                          : "Require approval"
                      }
                    >
                      <ShieldAlert className="h-3.5 w-3.5" />
                    </button>

                    {/* Enable/Disable Toggle */}
                    <button
                      onClick={() => handleSingleToggleEnabled(tool)}
                      disabled={pending || bulkBusy}
                      className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none shadow-sm ${
                        tool.is_enabled
                          ? "bg-primary/80 hover:bg-primary text-primary-foreground"
                          : "bg-muted hover:bg-muted/70"
                      } ${
                        pending ? "opacity-50 cursor-wait" : "cursor-pointer"
                      }`}
                      title={tool.is_enabled ? "Disable tool" : "Enable tool"}
                    >
                      <span
                        className={`inline-block h-5 w-5 transform rounded-full bg-background shadow transition-transform duration-200 ease-out ${
                          tool.is_enabled ? "translate-x-5" : "translate-x-1"
                        }`}
                      />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};
