import { useState } from "react";
import { PresetGrid } from "./PresetGrid";
import { ProviderForm } from "@/components/settings/ProviderForm";
import { PROVIDER_PRESETS, type ProviderPreset } from "@/lib/presets";
import { useSettingsData } from "@/hooks/useSettingsData";
import { useModelsContext } from "@/hooks/useModelsContext";
import { useAuth } from "@/hooks/useAuth";

/** Icon grid of known providers; picking one opens the pre-filled add form. */
export const ProviderPresetPicker = ({ className }: { className?: string }) => {
  const { addProvider, fetchAll } = useSettingsData();
  const { refreshModels } = useModelsContext();
  const { loginWithChatGPT } = useAuth();
  const [preset, setPreset] = useState<ProviderPreset | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [connectingId, setConnectingId] = useState<string | null>(null);

  const handleSelect = async (item: ProviderPreset) => {
    if (!item.oauth) {
      setPreset(item);
      setFormOpen(true);
      return;
    }
    setConnectingId(item.id);
    try {
      await loginWithChatGPT();
      await fetchAll();
      await refreshModels();
    } catch (err) {
      console.error("Failed to connect ChatGPT:", err);
    } finally {
      setConnectingId(null);
    }
  };

  return (
    <>
      <PresetGrid
        items={PROVIDER_PRESETS}
        onSelect={handleSelect}
        onOther={() => {
          setPreset(null);
          setFormOpen(true);
        }}
        busyId={connectingId}
        className={className}
      />
      <ProviderForm
        open={formOpen}
        onOpenChange={setFormOpen}
        onSubmit={addProvider}
        preset={preset}
        title={preset ? `Add ${preset.name}` : "Add AI Provider"}
        submitLabel="Add Provider"
      />
    </>
  );
};
