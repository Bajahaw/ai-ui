import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "../ui/card";
import {
  Database,
  Edit,
  ExternalLink,
  Loader2,
  Plus,
  RefreshCw,
  RotateCcw,
  Trash2,
} from "lucide-react";
import { ProviderForm } from "./ProviderForm";
import { FrontendProvider, ProviderRequest } from "@/lib/api/types";
import { useSettingsData } from "@/hooks/useSettingsData";
import { useModelsContext } from "@/hooks/useModelsContext";
import { useAuth } from "@/hooks/useAuth";

export const ProvidersSection = () => {
  const {
    data,
    addProvider,
    updateProvider,
    deleteProvider,
    getModelsByProvider,
    refreshProviderModels,
    fetchAll,
  } = useSettingsData();
  const { refreshModels } = useModelsContext();
  const { loginWithChatGPT } = useAuth();
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingProvider, setEditingProvider] =
    useState<FrontendProvider | null>(null);
  const [loadingModels, setLoadingModels] = useState(false);
  const [connectingChatGPT, setConnectingChatGPT] = useState(false);
  const [refreshingProviders, setRefreshingProviders] = useState<Set<string>>(
    new Set(),
  );

  const handleAddProvider = async (providerData: ProviderRequest) => {
    await addProvider(providerData);
    // addProvider already refreshes models internally
    setShowAddForm(false);
  };

  const handleEditProvider = async (providerData: ProviderRequest) => {
    if (editingProvider) {
      await updateProvider(providerData);
      setEditingProvider(null);
    }
  };

  const handleDeleteProvider = async (id: string) => {
    if (confirm("Are you sure you want to delete this provider?")) {
      await deleteProvider(id);
    }
  };

  const handleRefreshModels = async () => {
    setLoadingModels(true);
    try {
      await refreshModels();
    } finally {
      setLoadingModels(false);
    }
  };

  const handleRefreshProviderModels = async (id: string) => {
    setRefreshingProviders((prev) => new Set(prev).add(id));
    try {
      await refreshProviderModels(id);
    } finally {
      setRefreshingProviders((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }
  };

  const handleConnectChatGPT = async () => {
    setConnectingChatGPT(true);
    try {
      await loginWithChatGPT();
      await fetchAll();
      await refreshModels();
    } catch (err) {
      console.error("Failed to connect ChatGPT:", err);
    } finally {
      setConnectingChatGPT(false);
    }
  };

  const ChatGPTIcon = ({ className }: { className?: string }) => (
    <svg
      className={className}
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden
    >
      <path d="M22.282 9.821a5.985 5.985 0 0 0-.516-4.91 6.046 6.046 0 0 0-6.51-2.9A6.065 6.065 0 0 0 4.981 4.18a5.985 5.985 0 0 0-3.998 2.9 6.046 6.046 0 0 0 .743 7.097 5.98 5.98 0 0 0 .51 4.911 6.051 6.051 0 0 0 6.515 2.9A5.985 5.985 0 0 0 13.26 24a6.056 6.056 0 0 0 5.772-4.206 5.99 5.99 0 0 0 3.997-2.9 6.056 6.056 0 0 0-.747-7.073zM13.26 22.43a4.476 4.476 0 0 1-2.876-1.04l.141-.081 4.779-2.758a.795.795 0 0 0 .392-.681v-6.737l2.02 1.168a.071.071 0 0 1 .038.052v5.583a4.504 4.504 0 0 1-4.494 4.494zM3.6 18.304a4.47 4.47 0 0 1-.535-3.014l.142.085 4.783 2.759a.771.771 0 0 0 .78 0l5.843-3.369v2.332a.08.08 0 0 1-.033.062L9.74 19.95a4.5 4.5 0 0 1-6.14-1.646zM2.34 7.896a4.485 4.485 0 0 1 2.366-1.973V11.6a.766.766 0 0 0 .388.676l5.815 3.355-2.02 1.168a.076.076 0 0 1-.071 0l-4.83-2.786A4.504 4.504 0 0 1 2.34 7.872zm16.597 3.855l-5.833-3.387L15.119 7.2a.076.076 0 0 1 .071 0l4.83 2.791a4.494 4.494 0 0 1-.676 8.105v-5.678a.79.79 0 0 0-.395-.682zm2.01-3.023l-.141-.085-4.774-2.782a.776.776 0 0 0-.785 0L9.409 9.23V6.897a.066.066 0 0 1 .028-.061l4.83-2.787a4.5 4.5 0 0 1 6.68 4.66zm-12.64 4.135l-2.02-1.164a.08.08 0 0 1-.038-.057V6.075a4.5 4.5 0 0 1 7.375-3.453l-.142.08L8.704 5.46a.795.795 0 0 0-.393.681zm1.097-2.365l2.602-1.5 2.607 1.5v2.999l-2.597 1.5-2.607-1.5z" />
    </svg>
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium flex items-center gap-2">
          <Database className="h-5 w-5" />
          AI Providers
        </h3>
        <div className="flex items-center gap-2">
          <Button
            onClick={handleRefreshModels}
            variant="ghost"
            size="sm"
            disabled={loadingModels}
          >
            {loadingModels ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
          </Button>
          <Button
            onClick={handleConnectChatGPT}
            variant="outline"
            size="sm"
            disabled={connectingChatGPT}
            title="Add another ChatGPT account as a provider"
          >
            {connectingChatGPT ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <ChatGPTIcon className="h-4 w-4" />
            )}
            <span className="hidden sm:inline">Add ChatGPT</span>
          </Button>
          <Button
            onClick={() => setShowAddForm(true)}
            variant="outline"
            size="sm"
          >
            <Plus className="h-4 w-4" />
            <span className="hidden sm:inline">Add Provider</span>
          </Button>
        </div>
      </div>

      {data.providers.length === 0 ? (
        <Card className="p-6 text-center bg-transparent border-dashed">
          <div className="space-y-3">
            <p className="text-muted-foreground">No providers configured</p>
            <div className="flex flex-wrap items-center justify-center gap-2">
              <Button
                onClick={handleConnectChatGPT}
                variant="default"
                size="sm"
                disabled={connectingChatGPT}
              >
                {connectingChatGPT ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <ChatGPTIcon className="h-4 w-4" />
                )}
                Add ChatGPT
              </Button>
              <Button
                onClick={() => setShowAddForm(true)}
                variant="outline"
                size="sm"
              >
                <Plus className="h-4 w-4" />
                Add API Provider
              </Button>
            </div>
          </div>
        </Card>
      ) : (
        <div className="space-y-4 overflow-hidden">
          {data.providers.map((provider) => {
            const providerModels = getModelsByProvider(provider.id);
            return (
              <Card
                key={provider.id}
                className="p-4 bg-transparent overflow-hidden"
              >
                <div className="space-y-3">
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 min-w-0">
                        <h4 className="truncate max-w-[140px] sm:max-w-[300px]">
                          {provider.type === "chatgpt-oauth"
                            ? "ChatGPT"
                            : provider.id}
                        </h4>
                        {provider.type === "chatgpt-oauth" && (
                          <Badge variant="secondary" className="text-[10px] shrink-0">
                            OAuth
                          </Badge>
                        )}
                      </div>
                      {provider.type === "chatgpt-oauth" &&
                        provider.headers?.label && (
                          <p
                            className="text-xs text-muted-foreground truncate max-w-[200px] sm:max-w-[320px] mt-0.5"
                            title={provider.headers.label}
                          >
                            {provider.headers.label}
                          </p>
                        )}
                    </div>

                    <div className="flex items-center gap-1 flex-shrink-0">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleRefreshProviderModels(provider.id)}
                        disabled={refreshingProviders.has(provider.id)}
                        title="Refresh models from provider"
                      >
                        {refreshingProviders.has(provider.id) ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <RotateCcw className="h-4 w-4" />
                        )}
                      </Button>
                      {provider.type !== "chatgpt-oauth" && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setEditingProvider(provider)}
                          title="Edit provider"
                        >
                          <Edit className="h-4 w-4" />
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDeleteProvider(provider.id)}
                        className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-900/20"
                        title="Delete provider"
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <ExternalLink className="h-3 w-3 flex-shrink-0" />
                    <span
                      className="truncate max-w-[175px] sm:max-w-[300px]"
                      title={provider.baseUrl}
                    >
                      {provider.type === "chatgpt-oauth"
                        ? "ChatGPT account (subscription)"
                        : provider.baseUrl}
                    </span>
                  </div>

                  {providerModels.length > 0 && (
                    <div className="flex flex-wrap gap-1 overflow-hidden">
                      {providerModels.slice(0, 5).map((model) => (
                        <Badge
                          key={model.id}
                          variant="outline"
                          className="text-xs truncate max-w-[120px]"
                          title={model.name}
                        >
                          {model.name}
                        </Badge>
                      ))}
                      {providerModels.length > 5 && (
                        <Badge variant="outline" className="text-xs">
                          +{providerModels.length - 5} more
                        </Badge>
                      )}
                    </div>
                  )}
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {/* Add Provider Form */}
      <ProviderForm
        open={showAddForm}
        onOpenChange={setShowAddForm}
        onSubmit={handleAddProvider}
        title="Add AI Provider"
        submitLabel="Add Provider"
      />

      {/* Edit Provider Form */}
      <ProviderForm
        open={!!editingProvider}
        onOpenChange={(open) => !open && setEditingProvider(null)}
        onSubmit={handleEditProvider}
        provider={editingProvider}
        title="Edit AI Provider"
        submitLabel="Update Provider"
      />
    </div>
  );
};
