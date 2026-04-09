import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { AlertCircle, Loader2, Plus, Trash2 } from "lucide-react";
import { MCPServerRequest, MCPServerResponse } from "@/lib/api/types";

interface MCPServerFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: MCPServerRequest) => Promise<void>;
  server?: MCPServerResponse | null;
  title: string;
  submitLabel: string;
}

export const MCPServerForm = ({
  open,
  onOpenChange,
  onSubmit,
  server,
  title,
  submitLabel,
}: MCPServerFormProps) => {
  const [formData, setFormData] = useState<MCPServerRequest>({
    id: server?.id || "",
    name: server?.name || "",
    endpoint: server?.endpoint || "",
    api_key: "",
    headers: server?.headers || {},
  });

  const [headerEntries, setHeaderEntries] = useState<
    { key: string; value: string }[]
  >([]);

  useEffect(() => {
    if (server?.headers) {
      setHeaderEntries(
        Object.entries(server.headers).map(([key, value]) => ({ key, value })),
      );
    } else {
      setHeaderEntries([]);
    }
  }, [server]);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (!formData.name.trim()) {
      setError("Name is required");
      return;
    }

    if (!formData.endpoint.trim()) {
      setError("Endpoint is required");
      return;
    }

    // Validate URL format
    try {
      new URL(formData.endpoint);
    } catch {
      setError("Please enter a valid endpoint URL");
      return;
    }

    // Process headers
    const finalHeaders: Record<string, string> = {};
    for (const entry of headerEntries) {
      if (entry.key.trim() !== "") {
        finalHeaders[entry.key.trim()] = entry.value;
      }
    }
    const finalData = { ...formData, headers: finalHeaders };

    setIsSubmitting(true);
    try {
      await onSubmit(finalData);
      // Reset form and close dialog
      setFormData({ id: "", name: "", endpoint: "", api_key: "", headers: {} });
      setHeaderEntries([]);
      onOpenChange(false);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to save MCP server",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    setFormData({
      id: server?.id || "",
      name: server?.name || "",
      endpoint: server?.endpoint || "",
      api_key: "",
      headers: server?.headers || {},
    });
    if (server?.headers) {
      setHeaderEntries(
        Object.entries(server.headers).map(([key, value]) => ({ key, value })),
      );
    } else {
      setHeaderEntries([]);
    }
    setError(null);
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[425px] p-6 rounded-xl">
        <DialogHeader className="pb-2">
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="flex items-center gap-2 p-3 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              type="text"
              placeholder="My MCP Server"
              value={formData.name}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, name: e.target.value }))
              }
              disabled={isSubmitting}
              required
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="endpoint">Endpoint</Label>
            <Input
              id="endpoint"
              type="url"
              placeholder="https://mcp.example.com"
              value={formData.endpoint}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, endpoint: e.target.value }))
              }
              disabled={isSubmitting}
              required
            />
          </div>

          <div className="space-y-2.5">
            <div className="flex items-center justify-between">
              <Label>Custom Headers</Label>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() =>
                  setHeaderEntries([...headerEntries, { key: "", value: "" }])
                }
                disabled={isSubmitting}
              >
                <Plus className="h-4 w-4 mr-2" />
                Add Header
              </Button>
            </div>

            {headerEntries.length > 0 ? (
              <div className="space-y-1.5 max-h-[150px] overflow-y-auto pr-1">
                {headerEntries.map((header, index) => (
                  <div key={index} className="flex items-center gap-2">
                    <Input
                      placeholder="Key"
                      value={header.key}
                      onChange={(e) => {
                        const newEntries = [...headerEntries];
                        newEntries[index].key = e.target.value;
                        setHeaderEntries(newEntries);
                      }}
                      disabled={isSubmitting}
                    />
                    <Input
                      placeholder="Value"
                      value={header.value}
                      onChange={(e) => {
                        const newEntries = [...headerEntries];
                        newEntries[index].value = e.target.value;
                        setHeaderEntries(newEntries);
                      }}
                      disabled={isSubmitting}
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      onClick={() => {
                        const newEntries = [...headerEntries];
                        newEntries.splice(index, 1);
                        setHeaderEntries(newEntries);
                      }}
                      disabled={isSubmitting}
                    >
                      <Trash2 className="h-4 w-4 text-red-500" />
                    </Button>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground italic">
                No custom headers configured.
              </p>
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={handleCancel}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
