import { useState, useEffect, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Save, RotateCcw, FileText } from "lucide-react";
import { ModelSelect } from "@/components/ai-elements/model-select.tsx";
import { useSettingsData } from "@/hooks/useSettingsData";

export const DocumentsSection = () => {
    const { data, models, updateSettingsLocal, saveSettings } = useSettingsData();
    
    const [hasChanges, setHasChanges] = useState(false);
    const [isSaving, setSaving] = useState(false);

    const enabledModels = useMemo(() => 
        models.filter(m => m.is_enabled !== false),
        [models]
    );

    const attachmentOcrOnly = data.settings.attachmentOcrOnly === "true";
    const ocrModel = data.settings.ocrModel || "";

    const [local, setLocal] = useState({
        attachmentOcrOnly,
        ocrModel
    });

    useEffect(() => {
        setLocal({ attachmentOcrOnly, ocrModel });
        setHasChanges(false);
    }, [attachmentOcrOnly, ocrModel]);

    const handleToggleChange = () => {
        setLocal(prev => ({ ...prev, attachmentOcrOnly: !prev.attachmentOcrOnly }));
        setHasChanges(true);
    };

    const handleModelChange = (value: string) => {
        setLocal(prev => ({ ...prev, ocrModel: value }));
        setHasChanges(true);
    };

    const handleSave = async () => {
        setSaving(true);
        try {
            updateSettingsLocal("attachmentOcrOnly", local.attachmentOcrOnly.toString());
            if (local.ocrModel !== data.settings.ocrModel) {
                updateSettingsLocal("ocrModel", local.ocrModel);
            }
            await saveSettings();
            setHasChanges(false);
        } finally {
            setSaving(false);
        }
    };

    const handleReset = () => {
        setLocal({ attachmentOcrOnly, ocrModel });
        setHasChanges(false);
    };

    const isModelValid = enabledModels.some(m => m.id === local.ocrModel);

    return (
        <div className="space-y-8 max-w-2xl">
            <h3 className="text-lg font-medium flex items-center gap-2">
                <FileText className="h-5 w-5" />
                Document Settings
            </h3>

            <div className="space-y-4">
                <div className="flex justify-between items-center pb-2">
                    <div className="space-y-0.5">
                        <Label>
                            OCR Only Mode
                        </Label>
                        <p className="text-sm text-muted-foreground">
                            Pass attachments to OCR when uploading 
                        </p>
                    </div>
                    <Switch
                        checked={local.attachmentOcrOnly}
                        onCheckedChange={handleToggleChange}
                        disabled={isSaving}
                        title={local.attachmentOcrOnly ? "Disable OCR only mode" : "Enable OCR only mode"}
                    />
                </div>

                <div className="flex justify-between items-center pb-2">
                    <div className="space-y-0.5">
                        <Label htmlFor="ocr-model" className="text-nowrap">
                            OCR Model
                        </Label>
                        <p className="text-sm text-muted-foreground mr-4 hidden sm:block">
                            Select a model capable of processing attachments
                        </p>
                    </div>
                    <ModelSelect
                        models={enabledModels}
                        value={isModelValid ? local.ocrModel : undefined}
                        onChange={handleModelChange}
                        loading={false}
                        disabled={isSaving}
                        helperMessage="Select a model for OCR processing"
                        size="sm"
                        triggerId="ocr-model"
                        triggerAriaLabel="OCR model"
                        triggerClassName="max-sm:max-w-[180px] max-sm:mr-4"
                        contentClassName="max-h-60"
                        showCount={enabledModels.length > 0}
                    />
                </div>
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
    );
};
