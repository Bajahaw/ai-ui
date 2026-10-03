import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "../ui/card";
import {
  Edit,
  KeyRound,
  Loader2,
  Plus,
  RotateCcw,
  Server,
  Trash2,
} from "lucide-react";
import { MCPServerForm } from "./MCPServerForm";
import { MCPServerResponse } from "@/lib/api/types";
import { useSettingsData } from "@/hooks/useSettingsData";
import { isDefaultMCPServer } from "@/lib/onboarding";
import { authorizeMCPServer, openOAuthPopup } from "@/lib/mcpOAuth";
import { MCPPresetPicker } from "@/components/onboarding/MCPPresetPicker";

export const MCPServersSection = () => {
  const {
    data,
    addMCPServer,
    updateMCPServer,
    deleteMCPServer,
    refreshMCPTools,
    reloadMCPServers,
    restoreDefaultMCPServer,
  } = useSettingsData();
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingServer, setEditingServer] = useState<MCPServerResponse | null>(
    null,
  );
  const [refreshingServers, setRefreshingServers] = useState<Set<string>>(
    new Set(),
  );
  const [restoringDefault, setRestoringDefault] = useState(false);
  const [connecting, setConnecting] = useState<string | null>(null);
  const [connectErrors, setConnectErrors] = useState<Record<string, string>>(
    {},
  );

  const handleConnect = async (id: string) => {
    // Opened synchronously so the browser doesn't treat it as an unsolicited popup.
    const popup = openOAuthPopup();
    setConnecting(id);
    setConnectErrors((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });
    try {
      await authorizeMCPServer(id, popup);
      await reloadMCPServers();
    } catch (err) {
      setConnectErrors((prev) => ({
        ...prev,
        [id]: err instanceof Error ? err.message : "Authorization failed",
      }));
    } finally {
      setConnecting((current) => (current === id ? null : current));
    }
  };

  const handleDeleteServer = async (id: string) => {
    if (confirm("Are you sure you want to delete this MCP server?")) {
      await deleteMCPServer(id);
    }
  };

  const handleRefreshTools = async (id: string) => {
    setRefreshingServers((prev) => new Set(prev).add(id));
    try {
      await refreshMCPTools(id);
    } finally {
      setRefreshingServers((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }
  };

  const handleRestoreDefaultServer = async () => {
    setRestoringDefault(true);
    try {
      await restoreDefaultMCPServer();
    } finally {
      setRestoringDefault(false);
    }
  };

  const hasDefaultServer = data.mcpServers.some((s) =>
    isDefaultMCPServer(s.id),
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium flex items-center gap-2">
          <Server className="h-5 w-5" />
          MCP Servers
        </h3>
        <div className="flex items-center gap-2">
          {!hasDefaultServer && (
            <Button
              onClick={handleRestoreDefaultServer}
              variant="outline"
              size="sm"
              disabled={restoringDefault}
              title="Restore built-in default server"
            >
              {restoringDefault ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <RotateCcw className="h-4 w-4" />
              )}
              <span className="hidden sm:inline">Restore Default</span>
            </Button>
          )}
          <Button
            onClick={() => setShowAddForm(true)}
            variant="outline"
            size="sm"
          >
            <Plus className="h-4 w-4" />
            <span className="hidden sm:inline">Add Server</span>
          </Button>
        </div>
      </div>

      <Card className="p-3 bg-transparent border-dashed">
        <MCPPresetPicker />
      </Card>

      {data.mcpServers.length > 0 && (
        <div className="space-y-4 overflow-hidden">
          {data.mcpServers.map((server) => (
            <Card
              key={server.id}
              className="p-4 bg-transparent overflow-hidden"
            >
              <div className="space-y-3">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <h4 className="truncate max-w-[75px] sm:max-w-[300px]">
                      {server.name}
                    </h4>
                    {server.server_info && (
                      <p
                        className="text-xs text-muted-foreground truncate max-w-[75px] sm:max-w-[300px]"
                        title={`${server.server_info.name} ${server.server_info.version ?? ""}`.trim()}
                      >
                        {server.server_info.title || server.server_info.name}
                        {/* Drop semver build metadata ("v1+abc") for display. */}
                        {server.server_info.version &&
                          ` ${server.server_info.version.split("+")[0]}`}
                      </p>
                    )}
                  </div>

                  <div className="flex items-center gap-1 flex-shrink-0">
                    {server.auth_type === "oauth2" && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleConnect(server.id)}
                        disabled={connecting === server.id}
                        title={
                          server.oauth?.connected
                            ? "Reconnect with OAuth"
                            : "Connect with OAuth"
                        }
                      >
                        {connecting === server.id ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <KeyRound className="h-4 w-4" />
                        )}
                        {!server.oauth?.connected && (
                          <span className="hidden sm:inline">Connect</span>
                        )}
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleRefreshTools(server.id)}
                      disabled={refreshingServers.has(server.id)}
                      title="Refresh tools from MCP server"
                    >
                      {refreshingServers.has(server.id) ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <RotateCcw className="h-4 w-4" />
                      )}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setEditingServer(server)}
                      title="Edit MCP server"
                    >
                      <Edit className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDeleteServer(server.id)}
                      className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-900/20"
                      title="Delete MCP server"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>

                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Server className="h-3 w-3 flex-shrink-0" />
                  <span
                    className="truncate max-w-[175px] sm:max-w-[300px]"
                    title={server.endpoint}
                  >
                    {server.endpoint}
                  </span>
                  {server.auth_type === "oauth2" && (
                    <span className="flex items-center gap-1.5 flex-shrink-0 text-xs">
                      <span
                        className={`h-1.5 w-1.5 rounded-full ${server.oauth?.connected ? "bg-green-500" : "bg-amber-500"}`}
                      />
                      {server.oauth?.connected ? "OAuth" : "Not connected"}
                    </span>
                  )}
                  {server.update_pending && (
                    <Badge
                      variant="outline"
                      className="flex-shrink-0 text-xs border-amber-500/50 text-amber-600 dark:text-amber-400"
                      title="This server's tools changed. Refresh to apply."
                    >
                      Update available
                    </Badge>
                  )}
                </div>

                {(server.server_info?.description ||
                  server.server_info?.instructions) && (
                  <p
                    className="text-xs text-muted-foreground line-clamp-2 break-words"
                    title={server.server_info.instructions}
                  >
                    {server.server_info.description ||
                      server.server_info.instructions}
                  </p>
                )}

                {connectErrors[server.id] && (
                  <p className="text-xs text-red-600 break-words">
                    {connectErrors[server.id]}
                  </p>
                )}

                {server.tools && server.tools.length > 0 && (
                  <div className="flex flex-wrap gap-1 overflow-hidden">
                    {server.tools.slice(0, 5).map((tool) => (
                      <Badge
                        key={tool.id}
                        variant="outline"
                        className="text-xs truncate max-w-[120px]"
                        title={tool.description || tool.name}
                      >
                        {tool.name}
                      </Badge>
                    ))}
                    {server.tools.length > 5 && (
                      <Badge variant="outline" className="text-xs">
                        +{server.tools.length - 5} more
                      </Badge>
                    )}
                  </div>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Add MCP Server Form */}
      <MCPServerForm
        open={showAddForm}
        onOpenChange={setShowAddForm}
        onSubmit={addMCPServer}
        title="Add MCP Server"
        submitLabel="Add Server"
      />

      {/* Edit MCP Server Form */}
      <MCPServerForm
        open={!!editingServer}
        onOpenChange={(open) => !open && setEditingServer(null)}
        onSubmit={updateMCPServer}
        server={editingServer}
        title="Edit MCP Server"
        submitLabel="Update Server"
      />
    </div>
  );
};
