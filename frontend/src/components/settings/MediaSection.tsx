import { useState, useEffect, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Save, RotateCcw, ImageIcon } from "lucide-react";
import { ModelSelect } from "@/components/ai-elements/model-select.tsx";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useSettingsData } from "@/hooks/useSettingsData";

export const MediaSection = () => {
  const { data, models, updateSettingsLocal, saveSettings } = useSettingsData();

  const [hasChanges, setHasChanges] = useState(false);
  const [isSaving, setSaving] = useState(false);

  const enabledModels = useMemo(
    () => models.filter((m) => m.is_enabled !== false),
    [models],
  );

  const imageModel = data.settings.imageModel || "dall-e-3";
  const ttsModel = data.settings.ttsModel || "";
  const ttsVoice = data.settings.ttsVoice || "alloy";
  const ttsSpeed = data.settings.ttsSpeed || "1";

  const [local, setLocal] = useState({
    imageModel,
    ttsModel,
    ttsVoice,
    ttsSpeed,
  });

  useEffect(() => {
    setLocal({ imageModel, ttsModel, ttsVoice, ttsSpeed });
    setHasChanges(false);
  }, [imageModel, ttsModel, ttsVoice, ttsSpeed]);

  const handleChange = (key: string, value: string) => {
    setLocal((prev) => ({ ...prev, [key]: value }));
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
    setLocal({ imageModel, ttsModel, ttsVoice, ttsSpeed });
    setHasChanges(false);
  };

  const speedOptions = [
    { value: "0.25", label: "0.25×" },
    { value: "0.5", label: "0.5×" },
    { value: "0.75", label: "0.75×" },
    { value: "1", label: "1× (default)" },
    { value: "1.25", label: "1.25×" },
    { value: "1.5", label: "1.5×" },
    { value: "1.75", label: "1.75×" },
    { value: "2", label: "2×" },
    { value: "2.5", label: "2.5×" },
    { value: "3", label: "3×" },
    { value: "4", label: "4×" },
  ];

  const speedSelectValue = speedOptions.some((o) => o.value === local.ttsSpeed)
    ? local.ttsSpeed
    : "1";

  return (
    <div className="space-y-8 max-w-2xl">
      <h3 className="text-lg font-medium flex items-center gap-2">
        <ImageIcon className="h-5 w-5" />
        Media Settings
      </h3>

      <div className="space-y-4">
        <div className="flex justify-between items-center mb-0 pb-2">
          <Label htmlFor="image-model" className="text-nowrap">
            Image Model
          </Label>
          <ModelSelect
            models={enabledModels}
            value={
              enabledModels.some((m) => m.id === local.imageModel)
                ? local.imageModel
                : undefined
            }
            onChange={(value) => handleChange("imageModel", value)}
            loading={false}
            disabled={isSaving}
            helperMessage="Used for image generation tools"
            size="sm"
            triggerId="image-model"
            triggerAriaLabel="Image model"
            triggerClassName="max-sm:max-w-[180px] max-sm:mr-4"
            contentClassName="max-h-60"
            showCount={enabledModels.length > 0}
          />
        </div>

        <div className="flex justify-between items-center mb-0 pb-2">
          <Label htmlFor="tts-model" className="text-nowrap">
            TTS Model
          </Label>
          <ModelSelect
            models={enabledModels}
            value={
              enabledModels.some((m) => m.id === local.ttsModel)
                ? local.ttsModel
                : undefined
            }
            onChange={(value) => handleChange("ttsModel", value)}
            loading={false}
            disabled={isSaving}
            helperMessage="OpenAI-compatible speech model for read aloud"
            size="sm"
            triggerId="tts-model"
            triggerAriaLabel="TTS model"
            triggerClassName="max-sm:max-w-[180px] max-sm:mr-4"
            contentClassName="max-h-60"
            showCount={enabledModels.length > 0}
          />
        </div>

        <div className="flex justify-between items-center !my-0 pb-2">
          <Label htmlFor="tts-voice" className="text-nowrap">
            TTS Voice
          </Label>
          <Select
            value={local.ttsVoice}
            onValueChange={(value) => handleChange("ttsVoice", value)}
            disabled={isSaving}
          >
            <SelectTrigger
              id="tts-voice"
              className="flex items-center justify-between gap-2 rounded-lg !border-none !bg-transparent transition-colors"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="rounded-xl min-w-[120px] border border-border/70 p-1 shadow-xl">
              <SelectItem value="alloy">Alloy</SelectItem>
              <SelectItem value="ash">Ash</SelectItem>
              <SelectItem value="ballad">Ballad</SelectItem>
              <SelectItem value="coral">Coral</SelectItem>
              <SelectItem value="echo">Echo</SelectItem>
              <SelectItem value="fable">Fable</SelectItem>
              <SelectItem value="onyx">Onyx</SelectItem>
              <SelectItem value="nova">Nova</SelectItem>
              <SelectItem value="sage">Sage</SelectItem>
              <SelectItem value="shimmer">Shimmer</SelectItem>
              <SelectItem value="verse">Verse</SelectItem>
              <SelectItem value="marin">Marin</SelectItem>
              <SelectItem value="cedar">Cedar</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex justify-between items-center !my-0 pb-2">
          <Label htmlFor="tts-speed" className="text-nowrap">
            TTS Speed
          </Label>
          <Select
            value={speedSelectValue}
            onValueChange={(value) => handleChange("ttsSpeed", value)}
            disabled={isSaving}
          >
            <SelectTrigger
              id="tts-speed"
              className="flex items-center justify-between gap-2 rounded-lg !border-none !bg-transparent transition-colors"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="rounded-xl min-w-[140px] border border-border/70 p-1 shadow-xl">
              {speedOptions.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
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
