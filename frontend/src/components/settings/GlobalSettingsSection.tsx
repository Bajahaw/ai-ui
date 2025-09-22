import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { AlertCircle, Save, Loader2, RotateCcw } from "lucide-react";
import { useSettings } from "@/hooks/useSettings";
import { useModels } from "@/hooks/useModels";
import { useAutoSelectDefaultModel } from "@/hooks/useAutoSelectDefaultModel";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {cn} from "@/lib/utils.ts";

export const GlobalSettingsSection = () => {
  const {
    systemPrompt,
    isLoading,
    error,
    updateSystemPromptSetting,
    updateSingleSetting,
    getSingleSetting,
    clearError,
  } = useSettings();

  const { models, isLoading: modelsLoading } = useModels();

  // Auto-select default model when models become available
  const { autoSelectedModel } = useAutoSelectDefaultModel();

  const [localSystemPrompt, setLocalSystemPrompt] = useState("");
  const [localDefaultModel, setLocalDefaultModel] = useState("");
  const [isSaving, setSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const isLocalModelValid = models.some(
    (model) => model.id === localDefaultModel,
  );

  // Update local state when settings change
  useEffect(() => {
    setLocalSystemPrompt(systemPrompt);
    const savedModel = getSingleSetting("defaultModel");
    if (!modelsLoading && models.length === 0) {
      // When no models are available, clear the selection so the placeholder is rendered
      setLocalDefaultModel("");
    } else {
      setLocalDefaultModel(savedModel || autoSelectedModel || "");
    }
    setHasChanges(false);
  }, [
    systemPrompt,
    getSingleSetting,
    autoSelectedModel,
    models,
    modelsLoading,
  ]);

  const handleSystemPromptChange = (value: string) => {
    setLocalSystemPrompt(value);
    setHasChanges(
      value !== systemPrompt ||
        localDefaultModel !== getSingleSetting("defaultModel"),
    );
  };

  const handleDefaultModelChange = (value: string) => {
    setLocalDefaultModel(value);
    setHasChanges(
      localSystemPrompt !== systemPrompt ||
        value !== getSingleSetting("defaultModel"),
    );
  };

  const handleSaveSettings = async () => {
    setSaving(true);
    try {
      await updateSystemPromptSetting(localSystemPrompt);
      if (localDefaultModel) {
        await updateSingleSetting("defaultModel", localDefaultModel);
      }
      setHasChanges(false);
    } catch (error) {
      console.error("Failed to save settings:", error);
    } finally {
      setSaving(false);
    }
  };

  const handleResetSettings = () => {
    setLocalSystemPrompt(systemPrompt);
    setLocalDefaultModel(getSingleSetting("defaultModel") || "");
    setHasChanges(false);
  };

  const defaultSystemPrompt =
    "You are a helpful AI assistant. Provide clear, accurate, and helpful responses to user questions.";

  if (isLoading || modelsLoading) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-center py-12">
          <div className="flex items-center gap-3 text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin" />
            <span className="text-sm">
              {isLoading && modelsLoading
                ? "Loading settings and models..."
                : isLoading
                  ? "Loading settings..."
                  : "Loading models..."}
            </span>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-8 max-w-2xl">
      <div>
        <h2 className="text-xl font-semibold text-foreground mb-2">
          Global Settings
        </h2>
        <p className="text-sm text-muted-foreground">
          Configure default behavior for all conversations
        </p>
      </div>

      {error && (
        <div className="flex items-center gap-3 p-4 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
          <AlertCircle className="h-4 w-4 flex-shrink-0" />
          <span className="flex-1">{error}</span>
          <Button
            variant="ghost"
            size="sm"
            onClick={clearError}
            className="h-6 w-6 p-0 hover:bg-red-100 dark:hover:bg-red-800/30"
          >
            ✕
          </Button>
        </div>
      )}

      <div className="space-y-8">
        {/* System Prompt Section */}
        <div className="space-y-4">
          <div>
            <Label htmlFor="system-prompt" className="text-base font-medium">
              System Prompt
            </Label>
          </div>

          <Textarea
            id="system-prompt"
            placeholder={defaultSystemPrompt}
            value={localSystemPrompt}
            onChange={(e) => handleSystemPromptChange(e.target.value)}
            className="min-h-[140px] resize-y bg-transparent border-border/50 focus-visible:border-border"
            disabled={isSaving}
          />
        </div>

        {/* Default Model Section */}
        <div className="space-y-4">
          <div>
            <Label htmlFor="default-model" className="text-base font-medium">
              Default Model
            </Label>
          </div>

          <Select
            value={isLocalModelValid ? localDefaultModel : undefined}
            onValueChange={handleDefaultModelChange}
            disabled={isSaving || models.length === 0 || modelsLoading}
          >
            <SelectTrigger
              id="default-model"
              className="bg-transparent border-border/50 focus:border-border"
            >
              <SelectValue
                placeholder={
                  modelsLoading
                    ? "Loading models..."
                    : models.length === 0
                      ? "No models available"
                      : "Select a model"
                }
              />
            </SelectTrigger>
            <SelectContent className="max-h-60 overflow-y-auto">
              {modelsLoading ? (
                <div className="px-3 py-6 text-center">
                  <div className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    Loading models...
                  </div>
                </div>
              ) : models.length === 0 ? (
                <div className="px-3 py-6 text-center">
                  <div className="text-sm text-muted-foreground">
                    No models available
                  </div>
                  <div className="text-xs text-muted-foreground mt-1">
                    Add AI providers in the Providers tab
                  </div>
                </div>
              ) : (
                <>
                  {models.map((model) => (
                    <SelectItem
                        className={cn(
                            "cursor-pointer rounded-xl px-1 py-2.5 text-base mx-2 my-0.5",
                            "hover:bg-muted-foreground/10 hover:text-foreground",
                            "focus:bg-muted-foreground/10 focus:text-foreground",
                            "transition-all duration-0 ease-in-out",
                            "!ring-0 !outline-none focus:!ring-0 focus-visible:!ring-0",)}
                        key={model.id} value={model.id}>
                      {model.name}
                    </SelectItem>
                  ))}
                </>
              )}
            </SelectContent>
          </Select>
        </div>

        {/* Action Buttons */}
        <div className="flex items-center gap-3 pt-2">
          <Button
            onClick={handleSaveSettings}
            disabled={!hasChanges || isSaving}
            className="gap-2"
          >
            {isSaving && <Loader2 className="h-4 w-4 animate-spin" />}
            <Save className="h-4 w-4" />
            Apply
          </Button>

          {hasChanges && (
            <Button
              variant="outline"
              onClick={handleResetSettings}
              disabled={isSaving}
              className="gap-2"
            >
              <RotateCcw className="h-4 w-4" />
              Reset
            </Button>
          )}
        </div>
      </div>
    </div>
  );
};
