import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "../ui/card";
import { AlertCircle, Save, Loader2, RotateCcw } from "lucide-react";
import { useSettings } from "@/hooks/useSettings";
import { useModels } from "@/hooks/useModels";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

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

  const [localSystemPrompt, setLocalSystemPrompt] = useState("");
  const [localDefaultModel, setLocalDefaultModel] = useState("");
  const [isSaving, setSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);

  // Update local state when settings change
  useEffect(() => {
    setLocalSystemPrompt(systemPrompt);
    const savedModel = getSingleSetting("defaultModel");
    setLocalDefaultModel(savedModel || ""); // Handle case where setting doesn't exist yet
    setHasChanges(false);
  }, [systemPrompt, getSingleSetting]);

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
        // This will create the defaultModel setting if it doesn't exist
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
    setLocalDefaultModel(getSingleSetting("defaultModel") || ""); // Handle missing setting
    setHasChanges(false);
  };

  const defaultSystemPrompt =
    "You are a helpful AI assistant. Provide clear, accurate, and helpful responses to user questions.";

  if (isLoading || modelsLoading) {
    return (
      <div className="space-y-4">
        <h3 className="text-lg font-medium">Global Settings</h3>
        <div className="flex items-center justify-center py-8">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span>
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
    <div className="space-y-8">
      <h3 className="text-lg font-medium">Global Settings</h3>

      {error && (
        <div className="flex items-center gap-2 p-3 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
          <AlertCircle className="h-4 w-4 flex-shrink-0" />
          <span>{error}</span>
          <Button
            variant="ghost"
            size="sm"
            onClick={clearError}
            className="ml-auto"
          >
            ✕
          </Button>
        </div>
      )}

      <Card className="bg-transparent">
        <CardHeader className="pb-4">
          <CardTitle className="text-base">System Prompt</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-3">
            <Label htmlFor="system-prompt">
              Default system prompt for all conversations
            </Label>
            <Textarea
              id="system-prompt"
              placeholder={defaultSystemPrompt}
              value={localSystemPrompt}
              onChange={(e) => handleSystemPromptChange(e.target.value)}
              className="min-h-[120px] resize-y"
              disabled={isSaving}
            />
          </div>

          {hasChanges && (
            <div className="flex items-center gap-2 p-3 text-sm text-blue-600 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              <span>You have unsaved changes</span>
            </div>
          )}

          <div className="flex items-center gap-3">
            <Button
              onClick={handleSaveSettings}
              disabled={!hasChanges || isSaving}
              size="sm"
            >
              {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              <Save className="mr-2 h-4 w-4" />
              Save Changes
            </Button>

            {hasChanges && (
              <Button
                variant="outline"
                onClick={handleResetSettings}
                disabled={isSaving}
                size="sm"
              >
                <RotateCcw className="mr-2 h-4 w-4" />
                Reset
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      <Card className="bg-transparent">
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Default Model</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-3">
            <Label htmlFor="default-model">
              Default AI model for new conversations
            </Label>
            <Select
              value={localDefaultModel}
              onValueChange={handleDefaultModelChange}
              disabled={isSaving || models.length === 0 || modelsLoading}
            >
              <SelectTrigger id="default-model">
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
              <SelectContent>
                {modelsLoading ? (
                  <div className="px-3 py-4 text-center">
                    <div className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
                      <Loader2 className="h-3 w-3 animate-spin" />
                      Loading models...
                    </div>
                  </div>
                ) : models.length === 0 ? (
                  <div className="px-3 py-4 text-center">
                    <div className="text-sm text-muted-foreground">
                      No models available
                    </div>
                    <div className="text-xs text-muted-foreground mt-1">
                      Add AI providers in the Providers tab
                    </div>
                  </div>
                ) : (
                  <>
                    {!localDefaultModel && (
                      <SelectItem value="" disabled>
                        Select a default model
                      </SelectItem>
                    )}
                    {models.map((model) => (
                      <SelectItem key={model.id} value={model.id}>
                        {model.name}
                      </SelectItem>
                    ))}
                  </>
                )}
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};
