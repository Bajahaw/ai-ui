import { useState, useEffect, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Save, RotateCcw, Settings } from "lucide-react";
import { ModelSelect } from "@/components/ai-elements/model-select.tsx";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useSettingsData } from "@/hooks/useSettingsData";

export const GlobalSettingsSection = () => {
    const { data, models, updateSettingsLocal, saveSettings } = useSettingsData();
    
    const [hasChanges, setHasChanges] = useState(false);
    const [isSaving, setSaving] = useState(false);

    const enabledModels = useMemo(() => 
        models.filter(m => m.is_enabled !== false),
        [models]
    );

    const systemPrompt = data.settings.systemPrompt || "";
    const defaultModel = data.settings.defaultModel || "";
    const reasoningEffort = data.settings.reasoningEffort || "medium";
    const enterBehavior = data.settings.enterBehavior || "send";

    const [local, setLocal] = useState({
        systemPrompt,
        defaultModel,
        reasoningEffort,
        enterBehavior
    });

    useEffect(() => {
        setLocal({ systemPrompt, defaultModel, reasoningEffort, enterBehavior });
        setHasChanges(false);
    }, [systemPrompt, defaultModel, reasoningEffort, enterBehavior]);

    const handleChange = (key: string, value: string) => {
        setLocal(prev => ({ ...prev, [key]: value }));
        setHasChanges(true);
    };

    const handleSave = async () => {
        setSaving(true);
        try {
            Object.entries(local).forEach(([key, value]) => {
                if (value !== data.settings[key]) {
                    updateSettingsLocal(key, value);
                }
            });
            await saveSettings();
            setHasChanges(false);
        } finally {
            setSaving(false);
        }
    };

    const handleReset = () => {
        setLocal({ systemPrompt, defaultModel, reasoningEffort, enterBehavior });
        setHasChanges(false);
    };

    const isModelValid = enabledModels.some(m => m.id === local.defaultModel);

    return (
        <div className="space-y-8 max-w-2xl">
            <h3 className="text-lg font-medium flex items-center gap-2">
                <Settings className="h-5 w-5" />
                General Settings
            </h3>

            <div className="space-y-4">
                <div className="flex justify-between items-center pb-2">
                    <Label htmlFor="default-model" className="text-nowrap">
                        Default Model
                    </Label>
                    <ModelSelect
                        models={enabledModels}
                        value={isModelValid ? local.defaultModel : undefined}
                        onChange={(value) => handleChange("defaultModel", value)}
                        loading={false}
                        disabled={isSaving}
                        helperMessage="Add AI providers in the Providers tab"
                        size="sm"
                        triggerId="default-model"
                        triggerAriaLabel="Default model"
                        triggerClassName="max-sm:max-w-[180px] max-sm:mr-4"
                        contentClassName="max-h-60"
                        showCount={enabledModels.length > 0}
                    />
                </div>

                <div className="flex justify-between items-center !my-0 pb-2">
                    <Label htmlFor="reasoning-effort" className="text-nowrap">
                        Reasoning Effort
                    </Label>
                    <Select
                        value={local.reasoningEffort}
                        onValueChange={(value) => handleChange("reasoningEffort", value)}
                        disabled={isSaving}
                    >
                        <SelectTrigger
                            id="reasoning-effort"
                            className="flex items-center justify-between gap-2 rounded-lg !border-none !bg-transparent transition-colors"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent className="rounded-xl min-w-[120px] border border-border/70 p-1 shadow-xl">
                            <SelectItem value="disabled">Disabled</SelectItem>
                            <SelectItem value="none">None</SelectItem>
                            <SelectItem value="low">Low</SelectItem>
                            <SelectItem value="medium">Medium</SelectItem>
                            <SelectItem value="high">High</SelectItem>
                        </SelectContent>
                    </Select>
                </div>

                <div className="border-b border-border flex justify-between items-center !my-0 pb-2">
                    <Label htmlFor="enter-behavior" className="text-nowrap">
                        Enter Key Action
                    </Label>
                    <Select
                        value={local.enterBehavior}
                        onValueChange={(value) => handleChange("enterBehavior", value)}
                        disabled={isSaving}
                    >
                        <SelectTrigger
                            id="enter-behavior"
                            className="flex items-center justify-between gap-2 rounded-lg !border-none !bg-transparent transition-colors"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent className="rounded-xl min-w-[140px] border border-border/70 p-1 shadow-xl">
                            <SelectItem value="send">Send</SelectItem>
                            <SelectItem value="newline">New Line</SelectItem>
                        </SelectContent>
                    </Select>
                </div>

                <div className="space-y-4">
                    <Label htmlFor="system-prompt">
                        System Prompt
                    </Label>
                    <Textarea
                        id="system-prompt"
                        placeholder="You are a helpful AI assistant. Provide clear, accurate, and helpful responses to user questions."
                        value={local.systemPrompt}
                        onChange={(e) => handleChange("systemPrompt", e.target.value)}
                        className="min-h-[140px] text-sm resize-none !bg-secondary/50 rounded-lg border-border/80 focus-visible:border-border"
                        disabled={isSaving}
                    />
                </div>

                {hasChanges && (
                    <div className="flex gap-2 pt-4 border-t">
                        <Button
                            onClick={handleSave}
                            disabled={isSaving}
                            size="sm"
                            className="gap-2"
                        >
                            <Save className="h-4 w-4" />
                            {isSaving ? "Saving..." : "Save Changes"}
                        </Button>
                        <Button
                            onClick={handleReset}
                            disabled={isSaving}
                            variant="outline"
                            size="sm"
                            className="gap-2"
                        >
                            <RotateCcw className="h-4 w-4" />
                            Reset
                        </Button>
                    </div>
                )}
            </div>
        </div>
    );
};
