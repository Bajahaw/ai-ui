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
import { AlertCircle, Loader2, Eye, EyeOff, Plus, Trash2 } from "lucide-react";
import { ProviderRequest, FrontendProvider } from "@/lib/api/types";

interface ProviderFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: ProviderRequest) => Promise<void>;
  provider?: FrontendProvider | null;
  title: string;
  submitLabel: string;
}

export const ProviderForm = ({
  open,
  onOpenChange,
  onSubmit,
  provider,
  title,
  submitLabel,
}: ProviderFormProps) => {
  const [formData, setFormData] = useState<ProviderRequest>({
    base_url: provider?.baseUrl || "",
    api_key: "",
    headers: provider?.headers || {},
  });

  const [headerEntries, setHeaderEntries] = useState<
    { key: string; value: string }[]
  >([]);

  useEffect(() => {
    if (provider?.headers) {
      setHeaderEntries(
        Object.entries(provider.headers).map(([key, value]) => ({
          key,
          value,
        })),
      );
    } else {
      setHeaderEntries([]);
    }
  }, [provider]);

  const [showApiKey, setShowApiKey] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (!formData.base_url.trim()) {
      setError("Base URL is required");
      return;
    }

    if (!formData.api_key.trim()) {
      setError("API Key is required");
      return;
    }

    // Validate URL format
    try {
      new URL(formData.base_url);
    } catch {
      setError("Please enter a valid URL");
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
      setFormData({ base_url: "", api_key: "", headers: {} });
      setHeaderEntries([]);
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save provider");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    setFormData({
      base_url: provider?.baseUrl || "",
      api_key: "",
      headers: provider?.headers || {},
    });
    if (provider?.headers) {
      setHeaderEntries(
        Object.entries(provider.headers).map(([key, value]) => ({
          key,
          value,
        })),
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
            <Label htmlFor="base_url">Base URL</Label>
            <Input
              id="base_url"
              type="url"
              placeholder="https://api.example.com"
              value={formData.base_url}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, base_url: e.target.value }))
              }
              disabled={isSubmitting}
              required
            />
          </div>

          <div className="space-y-1.5">
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
