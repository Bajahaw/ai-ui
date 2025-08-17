import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "../ui/card";
import { AlertCircle, Save, Loader2, RotateCcw } from "lucide-react";
import { useSettings } from "@/hooks/useSettings";

export const GlobalSettingsSection = () => {
  const {
    systemPrompt,
    isLoading,
    error,
    updateSystemPromptSetting,
    clearError,
  } = useSettings();

  const [localSystemPrompt, setLocalSystemPrompt] = useState("");
  const [isSaving, setSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);

  // Update local state when system prompt changes
  useEffect(() => {
    setLocalSystemPrompt(systemPrompt);
    setHasChanges(false);
  }, [systemPrompt]);

  const handleSystemPromptChange = (value: string) => {
    setLocalSystemPrompt(value);
    setHasChanges(value !== systemPrompt);
  };

  const handleSaveSystemPrompt = async () => {
    setSaving(true);
    try {
      await updateSystemPromptSetting(localSystemPrompt);
      setHasChanges(false);
    } catch (error) {
      console.error("Failed to save system prompt:", error);
    } finally {
      setSaving(false);
    }
  };

  const handleResetSystemPrompt = () => {
    setLocalSystemPrompt(systemPrompt);
    setHasChanges(false);
  };

  const defaultSystemPrompt =
    "You are a helpful AI assistant. Provide clear, accurate, and helpful responses to user questions.";

  if (isLoading) {
    return (
      <div className="space-y-4">
        <h3 className="text-lg font-medium">Global Settings</h3>
        <div className="flex items-center justify-center py-8">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span>Loading settings...</span>
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
              onClick={handleSaveSystemPrompt}
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
                onClick={handleResetSystemPrompt}
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
    </div>
  );
};
