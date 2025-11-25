import {useState} from "react";
import {Button} from "@/components/ui/button";
import {Badge} from "@/components/ui/badge";
import {Card} from "../ui/card";
import {Edit, Plus, Server, Trash2,} from "lucide-react";
import {MCPServerForm} from "./MCPServerForm";
import {MCPServerRequest, MCPServerResponse} from "@/lib/api/types";
import {useSettingsData} from "@/hooks/useSettingsData";

export const MCPServersSection = () => {
  const { data, addMCPServer, updateMCPServer, deleteMCPServer } = useSettingsData();
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingServer, setEditingServer] = useState<MCPServerResponse | null>(null);

  const handleAddServer = async (serverData: MCPServerRequest) => {
    await addMCPServer(serverData);
    setShowAddForm(false);
  };

  const handleEditServer = async (serverData: MCPServerRequest) => {
    if (editingServer) {
      await updateMCPServer(serverData);
      setEditingServer(null);
    }
  };

  const handleDeleteServer = async (id: string) => {
    if (confirm("Are you sure you want to delete this MCP server?")) {
      await deleteMCPServer(id);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">MCP Servers</h3>
        <Button onClick={() => setShowAddForm(true)} variant="outline" size="sm">
          <Plus className="h-4 w-4" />
          <span className="hidden sm:inline">Add Server</span>
        </Button>
      </div>

      {data.mcpServers.length === 0 ? (
        <Card className="p-6 text-center bg-transparent border-dashed">
          <div className="space-y-2">
            <p className="text-muted-foreground">No MCP servers configured</p>
            <Button onClick={() => setShowAddForm(true)} variant="outline" size="sm">
              <Plus className="h-4 w-4" />
              Add Your First MCP Server
            </Button>
          </div>
        </Card>
      ) : (
        <div className="space-y-4 overflow-hidden">
          {data.mcpServers.map((server) => (
            <Card
              key={server.id}
              className="p-4 bg-transparent overflow-hidden"
            >
              <div className="space-y-3">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <h4 className="sm:font-medium truncate max-w-[75px] sm:max-w-[300px]">
                      {server.name}
                    </h4>
                  </div>

                  <div className="flex items-center gap-1 flex-shrink-0">
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
                </div>

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
        onSubmit={handleAddServer}
        title="Add MCP Server"
        submitLabel="Add Server"
      />

      {/* Edit MCP Server Form */}
      <MCPServerForm
        open={!!editingServer}
        onOpenChange={(open) => !open && setEditingServer(null)}
        onSubmit={handleEditServer}
        server={editingServer}
        title="Edit MCP Server"
        submitLabel="Update Server"
      />
    </div>
  );
};
