import { useState } from "react";
import { PresetGrid } from "./PresetGrid";
import { MCPServerForm } from "@/components/settings/MCPServerForm";
import { MCP_PRESETS, type MCPPreset } from "@/lib/presets";
import { useSettingsData } from "@/hooks/useSettingsData";

/** Icon grid of known MCP services; picking one opens the pre-filled add form. */
export const MCPPresetPicker = ({ className }: { className?: string }) => {
  const { addMCPServer } = useSettingsData();
  const [preset, setPreset] = useState<MCPPreset | null>(null);
  const [formOpen, setFormOpen] = useState(false);

  const open = (item: MCPPreset | null) => {
    setPreset(item);
    setFormOpen(true);
  };

  return (
    <>
      <PresetGrid
        items={MCP_PRESETS}
        onSelect={open}
        onOther={() => open(null)}
        className={className}
      />
      <MCPServerForm
        open={formOpen}
        onOpenChange={setFormOpen}
        onSubmit={addMCPServer}
        preset={preset}
        title={preset ? `Add ${preset.name}` : "Add MCP Server"}
        submitLabel="Add Server"
      />
    </>
  );
};
