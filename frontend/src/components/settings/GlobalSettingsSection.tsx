import {useState, useEffect} from "react";
import {Button} from "@/components/ui/button";
import {Textarea} from "@/components/ui/textarea";
import {Label} from "@/components/ui/label";
import {AlertCircle, Save, Loader2, RotateCcw} from "lucide-react";
import {useSettings} from "@/hooks/useSettings";
import {useModels} from "@/hooks/useModels";
import {ModelSelect} from "@/components/ai-elements/model-select.tsx";
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

    const {models, isLoading: modelsLoading} = useModels();

    // Auto-select default model when models become available

    const [localSystemPrompt, setLocalSystemPrompt] = useState("");
    const [localDefaultModel, setLocalDefaultModel] = useState("");
    const [localReasoningEffort, setLocalReasoningEffort] = useState("");
    const [isSaving, setSaving] = useState(false);
    const [hasChanges, setHasChanges] = useState(false);
    const isLocalModelValid = models.some(
        (model) => model.id === localDefaultModel,
    );

    // Update local state when settings change
    useEffect(() => {
        setLocalSystemPrompt(systemPrompt);
        const savedModel = getSingleSetting("defaultModel");
        const savedReasoningEffort = getSingleSetting("reasoningEffort");
        if (!modelsLoading && models.length === 0) {
            // When no models are available, clear the selection so the placeholder is rendered
            setLocalDefaultModel("");
        } else {
            setLocalDefaultModel(savedModel || "");
        }
        setLocalReasoningEffort(savedReasoningEffort || "medium");
        setHasChanges(false);
    }, [
        systemPrompt,
        getSingleSetting,
        models,
        modelsLoading,
    ]);

    const handleSystemPromptChange = (value: string) => {
        setLocalSystemPrompt(value);
        setHasChanges(
            value !== systemPrompt ||
            localDefaultModel !== getSingleSetting("defaultModel") ||
            localReasoningEffort !== getSingleSetting("reasoningEffort"),
        );
    };

    const handleDefaultModelChange = (value: string) => {
        setLocalDefaultModel(value);
        setHasChanges(
            localSystemPrompt !== systemPrompt ||
            value !== getSingleSetting("defaultModel") ||
            localReasoningEffort !== getSingleSetting("reasoningEffort"),
        );
    };

    const handleReasoningEffortChange = (value: string) => {
        setLocalReasoningEffort(value);
        setHasChanges(
            localSystemPrompt !== systemPrompt ||
            localDefaultModel !== getSingleSetting("defaultModel") ||
            value !== getSingleSetting("reasoningEffort"),
        );
    };

    const handleSaveSettings = async () => {
        setSaving(true);
        try {
            await updateSystemPromptSetting(localSystemPrompt);
            if (localDefaultModel) {
                await updateSingleSetting("defaultModel", localDefaultModel);
            }
            if (localReasoningEffort) {
                await updateSingleSetting("reasoningEffort", localReasoningEffort);
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
        setLocalReasoningEffort(getSingleSetting("reasoningEffort") || "medium");
        setHasChanges(false);
    };

    const defaultSystemPrompt =
        "You are a helpful AI assistant. Provide clear, accurate, and helpful responses to user questions.";

    if (isLoading || modelsLoading) {
        return (
            <div className="space-y-6">
                <div className="flex items-center justify-center py-12">
                    <div className="flex items-center gap-3 text-muted-foreground">
                        <Loader2 className="h-5 w-5 animate-spin"/>
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
            <h2 className="text-lg text-foreground mb-2">
                General Settings
            </h2>

            {error && (
                <div
                    className="flex items-center gap-3 p-4 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
                    <AlertCircle className="h-4 w-4 flex-shrink-0"/>
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

            <div className="space-y-4">

                {/* Default Model Section */}
                <div className="flex justify-between items-center pb-2">
                    <div>
                        <Label htmlFor="default-model" className="font-medium text-nowrap">
                            Default Model
                        </Label>
                    </div>
                    <div className="">
                        <ModelSelect
                            models={models}
                            value={isLocalModelValid ? localDefaultModel : null}
                            onChange={handleDefaultModelChange}
                            loading={modelsLoading}
                            disabled={isSaving}
                            helperMessage="Add AI providers in the Providers tab"
                            size="sm"
                            triggerId="default-model"
                            triggerAriaLabel="Default model"
                            triggerClassName="max-sm:max-w-[180px] max-sm:mr-4"
                            contentClassName="max-h-60"
                            showCount={models.length > 0}
                        />
                    </div>

                </div>

                {/* Reasoning Effort Section */}
                <div className="border-b border-border flex justify-between items-center !my-0 pb-2">
                    <div>
                        <Label htmlFor="reasoning-effort" className="font-medium text-nowrap">
                            Reasoning Effort
                        </Label>
                    </div>
                    <div>
                        <Select
                            value={localReasoningEffort}
                            onValueChange={handleReasoningEffortChange}
                            disabled={isSaving}
                        >
                            <SelectTrigger
                                id="reasoning-effort"
                                className="flex items-center justify-between gap-2 rounded-lg !border-none !bg-transparent transition-colors data-[placeholder]:text-muted-foreground"
                            >
                                <SelectValue placeholder="Select reasoning effort" />
                            </SelectTrigger>
                            <SelectContent
                                className="rounded-xl min-w-[120px] border border-border/70 p-1 shadow-xl">
                                <SelectItem value="low">Low</SelectItem>
                                <SelectItem value="medium">Medium</SelectItem>
                                <SelectItem value="high">High</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                </div>

                {/* System Prompt Section */}
                <div className="space-y-4">
                    <div>
                        <Label htmlFor="system-prompt" className="font-medium">
                            System Prompt
                        </Label>
                    </div>

                    <Textarea
                        id="system-prompt"
                        placeholder={defaultSystemPrompt}
                        value={localSystemPrompt}
                        onChange={(e) => handleSystemPromptChange(e.target.value)}
                        className="min-h-[140px] text-sm resize-none !bg-secondary/50 rounded-lg border-border/80 focus-visible:border-border"
                        disabled={isSaving}
                    />
                </div>

                {/* Action Buttons */}
                <div className="flex items-center gap-3 pt-2">
                    <Button
                        onClick={handleSaveSettings}
                        disabled={!hasChanges || isSaving}
                        className="gap-2"
                    >
                        {isSaving && <Loader2 className="h-4 w-4 animate-spin"/>}
                        <Save className="h-4 w-4"/>
                        Apply
                    </Button>

                    {hasChanges && (
                        <Button
                            variant="outline"
                            onClick={handleResetSettings}
                            disabled={isSaving}
                            className="gap-2"
                        >
                            <RotateCcw className="h-4 w-4"/>
                            Reset
                        </Button>
                    )}
                </div>
            </div>
        </div>
    );
};
