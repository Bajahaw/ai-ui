import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  AlertCircle,
  Check,
  ChevronRight,
  Copy,
  Eye,
  EyeOff,
  Loader2,
  Plus,
  Trash2,
} from "lucide-react";
import {
  MCPAuthType,
  MCPOAuthRequest,
  MCPServerRequest,
  MCPServerResponse,
} from "@/lib/api/types";
import { getMCPOAuthRedirectURL } from "@/lib/api/mcpServers";
import { authorizeMCPServer, openOAuthPopup } from "@/lib/mcpOAuth";
import { useSettingsData } from "@/hooks/useSettingsData";
import type { MCPPreset } from "@/lib/presets";

const OAUTH_CALLBACK_PATH = "/api/tools/mcp/oauth/callback";

interface MCPServerFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: MCPServerRequest) => Promise<MCPServerResponse>;
  server?: MCPServerResponse | null;
  /** Pre-fills the form for a known service; ignored when editing. */
  preset?: MCPPreset | null;
  title: string;
  submitLabel: string;
}

type Phase = "idle" | "saving" | "authorizing";

export const MCPServerForm = ({
  open,
  onOpenChange,
  onSubmit,
  server,
  preset,
  title,
  submitLabel,
}: MCPServerFormProps) => {
  const { reloadMCPServers } = useSettingsData();

  const initialData = (): MCPServerRequest => ({
    id: server?.id || "",
    name: server?.name || preset?.name || "",
    endpoint: server?.endpoint || preset?.endpoint || "",
    api_key: "",
    headers: server?.headers || preset?.headers || {},
    auth_type: server ? (server.auth_type ?? "") : (preset?.authType ?? ""),
    auto_update: server?.auto_update ?? false,
  });
  const initialOAuth = (): MCPOAuthRequest => ({
    client_id: server?.oauth?.client_id ?? "",
    client_secret: "",
    scopes: server?.oauth?.scopes ?? "",
    auth_url: server?.oauth?.auth_url ?? "",
    token_url: server?.oauth?.token_url ?? "",
  });
  const initialHeaders = () =>
    Object.entries(server?.headers ?? preset?.headers ?? {}).map(
      ([key, value]) => ({ key, value }),
    );

  const [formData, setFormData] = useState<MCPServerRequest>(initialData);
  const [oauth, setOAuth] = useState<MCPOAuthRequest>(initialOAuth);
  const [headerEntries, setHeaderEntries] = useState<
    { key: string; value: string }[]
  >(initialHeaders);
  const [showApiKey, setShowApiKey] = useState(false);
  const [showSecret, setShowSecret] = useState(false);
  const [redirectURL, setRedirectURL] = useState("");
  const [copied, setCopied] = useState(false);
  const [phase, setPhase] = useState<Phase>("idle");
  const [error, setError] = useState<string | null>(null);

  const isOAuth = formData.auth_type === "oauth2";
  const isSubmitting = phase !== "idle";

  // Re-seed whenever the dialog opens so a stale server/preset never leaks in.
  useEffect(() => {
    if (!open) return;
    setFormData(initialData());
    setOAuth(initialOAuth());
    setHeaderEntries(initialHeaders());
    setError(null);
    setPhase("idle");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, server, preset]);

  useEffect(() => {
    if (!open || !isOAuth || redirectURL) return;
    getMCPOAuthRedirectURL()
      .then(setRedirectURL)
      .catch(() =>
        setRedirectURL(window.location.origin + OAUTH_CALLBACK_PATH),
      );
  }, [open, isOAuth, redirectURL]);

  const PresetIcon = server ? null : preset?.icon;

  const setOAuthField = (key: keyof MCPOAuthRequest, value: string) =>
    setOAuth((prev) => ({ ...prev, [key]: value }));

  // Mirrors mergeMCPOAuth on the backend: any change to these drops the stored token.
  const needsAuthorization = (): boolean => {
    if (!isOAuth) return false;
    const prev = server?.oauth;
    if (!server || server.auth_type !== "oauth2" || !prev?.connected) return true;
    return (
      formData.endpoint.trim() !== server.endpoint ||
      oauth.client_id.trim() !== prev.client_id ||
      oauth.client_secret !== "" ||
      oauth.scopes.trim() !== prev.scopes ||
      oauth.auth_url.trim() !== prev.auth_url ||
      oauth.token_url.trim() !== prev.token_url
    );
  };

  const copyRedirectURL = async () => {
    try {
      await navigator.clipboard.writeText(redirectURL);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard unavailable (e.g. plain HTTP); the URL is still selectable.
    }
  };

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
      if (isOAuth && oauth.auth_url.trim()) new URL(oauth.auth_url.trim());
      if (isOAuth && oauth.token_url.trim()) new URL(oauth.token_url.trim());
    } catch {
      setError("Please enter valid URLs");
      return;
    }

    // Process headers
    const finalHeaders: Record<string, string> = {};
    for (const entry of headerEntries) {
      if (entry.key.trim() !== "") {
        finalHeaders[entry.key.trim()] = entry.value;
      }
    }
    const finalData: MCPServerRequest = {
      ...formData,
      headers: finalHeaders,
      oauth: isOAuth ? oauth : undefined,
    };

    // The popup has to open inside the click, before any await.
    const popup = needsAuthorization() ? openOAuthPopup() : null;

    setPhase("saving");
    try {
      const saved = await onSubmit(finalData);
      // A retry after a failed authorization must update, not duplicate.
      setFormData((prev) => ({ ...prev, id: saved.id }));
      if (saved.auth_type === "oauth2" && !saved.oauth?.connected) {
        setPhase("authorizing");
        await authorizeMCPServer(saved.id, popup);
        await reloadMCPServers();
      } else {
        popup?.close();
      }
      // Reset form and close dialog
      setFormData({ id: "", name: "", endpoint: "", api_key: "", headers: {} });
      setHeaderEntries([]);
      onOpenChange(false);
    } catch (err) {
      popup?.close();
      setError(
        err instanceof Error ? err.message : "Failed to save MCP server",
      );
    } finally {
      setPhase("idle");
    }
  };

  const handleCancel = () => {
    setError(null);
    onOpenChange(false);
  };

  const secretPlaceholder =
    server?.oauth?.has_client_secret &&
    oauth.client_id.trim() === server.oauth.client_id
      ? "Leave blank to keep current secret"
      : "Optional for public clients";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[460px] max-h-[90vh] overflow-y-auto p-6 rounded-xl">
        <DialogHeader className="pb-2">
          <DialogTitle className="flex items-center gap-2">
            {PresetIcon && <PresetIcon className="h-5 w-5" />}
            {title}
          </DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="flex items-center gap-2 p-3 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span className="break-words min-w-0">{error}</span>
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

          <div className="space-y-1.5">
            <Label htmlFor="mcp_auth_type">Authentication</Label>
            <Select
              value={formData.auth_type || "none"}
              onValueChange={(value) =>
                setFormData((prev) => ({
                  ...prev,
                  auth_type: (value === "none" ? "" : value) as MCPAuthType,
                }))
              }
              disabled={isSubmitting}
            >
              <SelectTrigger id="mcp_auth_type" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent className="rounded-xl border border-border/70 p-1 shadow-xl">
                <SelectItem value="none">API key / headers</SelectItem>
                <SelectItem value="oauth2">OAuth2</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {isOAuth ? (
            <div className="space-y-4">
              {server?.auth_type === "oauth2" && (
                <p className="flex items-center gap-2 text-sm text-muted-foreground">
                  <span
                    className={`h-2 w-2 rounded-full ${server.oauth?.connected ? "bg-green-500" : "bg-amber-500"}`}
                  />
                  {server.oauth?.connected ? "Connected" : "Not connected"}
                </p>
              )}

              <div className="space-y-1.5">
                <Label htmlFor="mcp_redirect_url">OAuth Redirect URL</Label>
                <div className="flex items-center gap-2">
                  <Input
                    id="mcp_redirect_url"
                    readOnly
                    value={redirectURL}
                    onFocus={(e) => e.target.select()}
                    className="font-mono text-xs"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={copyRedirectURL}
                    disabled={!redirectURL}
                    title="Copy redirect URL"
                  >
                    {copied ? (
                      <Check className="h-4 w-4" />
                    ) : (
                      <Copy className="h-4 w-4" />
                    )}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">
                  Use this as the callback / redirect URL when creating your
                  OAuth app.
                </p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="mcp_client_id">Client ID</Label>
                <Input
                  id="mcp_client_id"
                  placeholder="Leave blank to register automatically"
                  value={oauth.client_id}
                  onChange={(e) => setOAuthField("client_id", e.target.value)}
                  disabled={isSubmitting}
                  autoComplete="off"
                />
                <p className="text-xs text-muted-foreground">
                  Blank works only if the server supports dynamic client
                  registration.
                </p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="mcp_client_secret">Client Secret</Label>
                <div className="relative">
                  <Input
                    id="mcp_client_secret"
                    type={showSecret ? "text" : "password"}
                    placeholder={secretPlaceholder}
                    value={oauth.client_secret}
                    onChange={(e) =>
                      setOAuthField("client_secret", e.target.value)
                    }
                    disabled={isSubmitting}
                    autoComplete="off"
                    className="pr-10"
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="absolute right-0 top-0 h-full px-3 py-2 hover:bg-transparent"
                    onClick={() => setShowSecret(!showSecret)}
                    disabled={isSubmitting}
                  >
                    {showSecret ? (
                      <EyeOff className="h-4 w-4" />
                    ) : (
                      <Eye className="h-4 w-4" />
                    )}
                    <span className="sr-only">
                      {showSecret ? "Hide" : "Show"} client secret
                    </span>
                  </Button>
                </div>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="mcp_scopes">Scopes</Label>
                <Input
                  id="mcp_scopes"
                  placeholder="Space-separated; blank uses server defaults"
                  value={oauth.scopes}
                  onChange={(e) => setOAuthField("scopes", e.target.value)}
                  disabled={isSubmitting}
                  autoComplete="off"
                />
              </div>

              <Collapsible
                defaultOpen={!!(oauth.auth_url || oauth.token_url)}
                className="space-y-3"
              >
                <CollapsibleTrigger className="group flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
                  <ChevronRight className="h-4 w-4 transition-transform group-data-[state=open]:rotate-90" />
                  Custom endpoints
                </CollapsibleTrigger>
                <CollapsibleContent className="space-y-3">
                  <div className="space-y-1.5">
                    <Label htmlFor="mcp_auth_url">Authorization URL</Label>
                    <Input
                      id="mcp_auth_url"
                      type="url"
                      placeholder="Discovered automatically"
                      value={oauth.auth_url}
                      onChange={(e) => setOAuthField("auth_url", e.target.value)}
                      disabled={isSubmitting}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="mcp_token_url">Access Token URL</Label>
                    <Input
                      id="mcp_token_url"
                      type="url"
                      placeholder="Discovered automatically"
                      value={oauth.token_url}
                      onChange={(e) =>
                        setOAuthField("token_url", e.target.value)
                      }
                      disabled={isSubmitting}
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Only needed when the server doesn't publish OAuth metadata.
                  </p>
                </CollapsibleContent>
              </Collapsible>
            </div>
          ) : (
            <div className="space-y-1.5">
              <Label htmlFor="mcp_api_key">API Key</Label>
              <div className="relative">
                <Input
                  id="mcp_api_key"
                  type={showApiKey ? "text" : "password"}
                  placeholder={
                    server
                      ? "Leave blank to keep current key"
                      : "Optional — sent as a Bearer token"
                  }
                  value={formData.api_key}
                  onChange={(e) =>
                    setFormData((prev) => ({
                      ...prev,
                      api_key: e.target.value,
                    }))
                  }
                  disabled={isSubmitting}
                  autoComplete="off"
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
          )}

          <div className="flex items-start justify-between gap-4 pt-2">
            <div className="space-y-1">
              <Label className="!mb-0">Auto-update tools</Label>
              <p className="text-xs text-muted-foreground">
                Apply tool changes from this server when the app opens
                (checked at most every 6 hours). Off: you'll see "Update
                available" and can refresh manually.
              </p>
            </div>
            <Switch
              className="mx-1 mt-0.5 flex-shrink-0"
              title="Auto-update tools"
              checked={!!formData.auto_update}
              onCheckedChange={() =>
                setFormData((prev) => ({
                  ...prev,
                  auto_update: !prev.auto_update,
                }))
              }
              disabled={isSubmitting}
            />
          </div>

          <div className="space-y-2.5">
            <div className="flex items-center justify-between pt-2">
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
              <div className="space-y-1.5 max-h-[150px] overflow-y-auto">
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
                      placeholder={
                        server?.headers?.[header.key] !== undefined
                          ? "Leave blank to keep"
                          : "Value"
                      }
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
              disabled={phase === "saving"}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              {phase === "authorizing"
                ? "Waiting for authorization…"
                : needsAuthorization()
                  ? "Save & Connect"
                  : submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
