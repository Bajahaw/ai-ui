import { useState } from "react";
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
import { AlertCircle, Loader2, Eye, EyeOff } from "lucide-react";
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
  });
  const [showApiKey, setShowApiKey] = useState(false);
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

    if (!formData.api_key.trim()) {
      setError("API Key is required");
      return;
    }

    // Validate URL format
    try {
      new URL(formData.endpoint);
    } catch {
      setError("Please enter a valid endpoint URL");
      return;
    }

    setIsSubmitting(true);
    try {
      await onSubmit(formData);
      // Reset form and close dialog
      setFormData({ id: "", name: "", endpoint: "", api_key: "" });
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
    });
    setError(null);
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[425px] rounded-xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="flex items-center gap-2 p-3 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div className="space-y-2 mt-4">
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

          <div className="space-y-2">
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

          <div className="space-y-2">
            <Label htmlFor="api_key">API Key</Label>
            <div className="relative">
              <Input
                id="api_key"
                type={showApiKey ? "text" : "password"}
                placeholder="Enter your API key"
                value={formData.api_key}
                onChange={(e) =>
                  setFormData((prev) => ({ ...prev, api_key: e.target.value }))
                }
                disabled={isSubmitting}
                required
                className="pr-10"
              />
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="absolute right-0 top-0 h-full px-3 py-2 hover:bg-transparent"
                onClick={() => setShowApiKey(!showApiKey)}
                disabled={isSubmitting}
              >
                {showApiKey ? (
                  <EyeOff className="h-4 w-4" />
                ) : (
                  <Eye className="h-4 w-4" />
                )}
                <span className="sr-only">
                  {showApiKey ? "Hide" : "Show"} API key
                </span>
              </Button>
            </div>
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
