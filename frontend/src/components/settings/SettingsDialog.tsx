import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import { ScrollArea } from "@/components/ui/scroll-area";
import { Settings, Database, ListChecks, Server, Wrench } from "lucide-react";
import { ProvidersSection } from "./ProvidersSection";
import { GlobalSettingsSection } from "./GlobalSettingsSection";
import { ModelsSection } from "./ModelsSection";
import { MCPServersSection } from "./MCPServersSection";
import { ToolsSection } from "./ToolsSection";

interface SettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type SettingsTab = "providers" | "models" | "global" | "mcp" | "tools";

export const SettingsDialog = ({ open, onOpenChange }: SettingsDialogProps) => {
  const [activeTab, setActiveTab] = useState<SettingsTab>("global");

  const tabs = [
    {
      id: "global" as const,
      label: "General",
      icon: Settings,
    },
    {
      id: "providers" as const,
      label: "Providers",
      icon: Database,
    },

    {
      id: "models" as const,
      label: "Models",
      icon: ListChecks,
    },
    {
      id: "mcp" as const,
      label: "MCP Servers",
      icon: Server,
    },
    {
      id: "tools" as const,
      label: "Tools",
      icon: Wrench,
    },
  ];

  const renderTabContent = () => {
    switch (activeTab) {
      case "providers":
        return <ProvidersSection />;
      case "models":
        return <ModelsSection />;
      case "global":
        return <GlobalSettingsSection />;
      case "mcp":
        return <MCPServersSection />;
      case "tools":
        return <ToolsSection />;
      default:
        return null;
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[calc(100%-1rem)] sm:w-[calc(100%-3rem)] max-w-7xl sm:max-w-4xl h-[65vh] p-0 flex flex-col rounded-xl">
        <DialogHeader className="px-6 py-4 border-b flex-shrink-0">
          <DialogTitle className="flex items-center gap-2">
            <Settings className="h-5 w-5" />
            Settings
          </DialogTitle>
        </DialogHeader>

        <div className="flex flex-1 min-h-0 overflow-hidden">
          {/* Sidebar */}
          <div className="w-16 sm:w-56 border-r flex-shrink-0">
            <div className="p-3 sm:p-4 md:p-5 lg:p-6 h-full">
              <nav className="space-y-2">
                {tabs.map((tab) => {
                  const Icon = tab.icon;
                  return (
                    <button
                      key={tab.id}
                      onClick={() => setActiveTab(tab.id)}
                      className={`w-full flex items-center justify-center sm:justify-start gap-2 sm:gap-3 px-2 sm:px-3 md:px-4 py-2 sm:py-3 text-left rounded-lg transition-colors ${
                        activeTab === tab.id
                          ? "bg-primary text-primary-foreground"
                          : "hover:bg-muted text-muted-foreground hover:text-foreground"
                      }`}
                    >
                      <Icon className="h-4 w-4 flex-shrink-0" />
                      <div className="text-xs sm:text-sm font-medium truncate hidden sm:block">
                        {tab.label}
                      </div>
                    </button>
                  );
                })}
              </nav>
            </div>
          </div>

          {/* Main Content */}
          <div className="flex-1 min-w-0 overflow-hidden">
            <ScrollArea className="h-full w-full">
              <div className="p-4 sm:p-8 max-w-full">{renderTabContent()}</div>
            </ScrollArea>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
};
