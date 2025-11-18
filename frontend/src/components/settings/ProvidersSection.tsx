import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "../ui/card";
import {
  Trash2,
  Edit,
  Plus,
  RefreshCw,
  ExternalLink,
  Loader2,
} from "lucide-react";
import { ProviderForm } from "./ProviderForm";
import { FrontendProvider, ProviderRequest } from "@/lib/api/types";
import { useSettingsData } from "@/hooks/useSettingsData";

export const ProvidersSection = () => {
  const { data, addProvider, updateProvider, deleteProvider, loadProviderModels } = useSettingsData();
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingProvider, setEditingProvider] = useState<FrontendProvider | null>(null);
  const [loadingModels, setLoadingModels] = useState<string | null>(null);

  const handleAddProvider = async (providerData: ProviderRequest) => {
    await addProvider(providerData);
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

  const handleLoadModels = async (providerId: string) => {
    setLoadingModels(providerId);
    try {
      await loadProviderModels(providerId);
    } finally {
      setLoadingModels(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">AI Providers</h3>
        <Button onClick={() => setShowAddForm(true)} variant="outline" size="sm">
          <Plus className="h-4 w-4" />
          <span className="hidden sm:inline">Add Provider</span>
        </Button>
      </div>

      {data.providers.length === 0 ? (
        <Card className="p-6 text-center bg-transparent border-dashed">
          <div className="space-y-2">
            <p className="text-muted-foreground">No providers configured</p>
            <Button
              onClick={() => setShowAddForm(true)}
              variant="outline"
              size="sm"
            >
              <Plus className="h-4 w-4" />
              Add Your First Provider
            </Button>
          </div>
        </Card>
      ) : (
        <div className="space-y-4 overflow-hidden">
          {data.providers.map((provider) => (
            <Card
              key={provider.id}
              className="p-4 bg-transparent overflow-hidden"
            >
              <div className="space-y-3">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <h4 className="sm:font-medium truncate max-w-[75px] sm:max-w-[300px]">
                      {provider.id}
                    </h4>
                  </div>

                  <div className="flex items-center gap-1 flex-shrink-0">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleLoadModels(provider.id)}
                      disabled={loadingModels === provider.id}
                      title="Refresh models"
                    >
                      {loadingModels === provider.id ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <RefreshCw className="h-4 w-4" />
                      )}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setEditingProvider(provider)}
                      title="Edit provider"
                    >
                      <Edit className="h-4 w-4" />
                    </Button>
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
                    {provider.baseUrl}
                  </span>
                </div>

                {provider.models && provider.models.length > 0 && (
                  <div className="flex flex-wrap gap-1 overflow-hidden">
                    {provider.models.slice(0, 5).map((model) => (
                      <Badge
                        key={model.id}
                        variant="outline"
                        className="text-xs truncate max-w-[120px]"
                        title={model.name}
                      >
                        {model.name}
                      </Badge>
                    ))}
                    {provider.models.length > 5 && (
                      <Badge variant="outline" className="text-xs">
                        +{provider.models.length - 5} more
                      </Badge>
                    )}
                  </div>
                )}
              </div>
            </Card>
          ))}
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

export default ProvidersSection;
