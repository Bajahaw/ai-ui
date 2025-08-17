import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import { ScrollArea } from "@/components/ui/scroll-area";
import { Settings, Database } from "lucide-react";
import { ProvidersSection } from "./ProvidersSection";
import { GlobalSettingsSection } from "./GlobalSettingsSection";

interface SettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type SettingsTab = "providers" | "global";

export const SettingsDialog = ({ open, onOpenChange }: SettingsDialogProps) => {
  const [activeTab, setActiveTab] = useState<SettingsTab>("providers");

  const tabs = [
    {
      id: "providers" as const,
      label: "AI Providers",
      icon: Database,
    },
    {
      id: "global" as const,
      label: "Global Settings",
      icon: Settings,
    },
  ];

  const renderTabContent = () => {
    switch (activeTab) {
      case "providers":
        return <ProvidersSection />;
      case "global":
        return <GlobalSettingsSection />;
      default:
        return null;
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-full max-w-[90vw] min-w-0 sm:min-w-[600px] md:min-w-[700px] lg:min-w-[800px] xl:min-w-[900px] h-[75vh] sm:h-[80vh] md:h-[85vh] p-0 flex flex-col">
        <DialogHeader className="px-6 py-4 border-b flex-shrink-0">
          <DialogTitle className="flex items-center gap-2">
            <Settings className="h-5 w-5" />
            Settings
          </DialogTitle>
        </DialogHeader>

        <div className="flex flex-1 min-h-0 overflow-hidden">
          {/* Sidebar */}
          <div className="w-48 sm:w-56 md:w-64 lg:w-72 border-r flex-shrink-0">
            <div className="p-3 sm:p-4 md:p-5 lg:p-6 h-full">
              <nav className="space-y-2">
                {tabs.map((tab) => {
                  const Icon = tab.icon;
                  return (
                    <button
                      key={tab.id}
                      onClick={() => setActiveTab(tab.id)}
                      className={`w-full flex items-center gap-2 sm:gap-3 px-2 sm:px-3 md:px-4 py-2 sm:py-3 text-left rounded-lg transition-colors ${
                        activeTab === tab.id
                          ? "bg-primary text-primary-foreground"
                          : "hover:bg-muted text-muted-foreground hover:text-foreground"
                      }`}
                    >
                      <Icon className="h-4 w-4 flex-shrink-0" />
                      <div className="text-xs sm:text-sm font-medium truncate">
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
              <div className="p-8 max-w-full">{renderTabContent()}</div>
            </ScrollArea>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
};
