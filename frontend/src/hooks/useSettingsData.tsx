import {createContext, ReactNode, useCallback, useContext, useRef, useState} from "react";
import {
  backendToFrontendProvider,
  deleteProvider,
  getProviders,
  saveProvider
} from "@/lib/api/providers";
import {deleteMCPServer as deleteMCPServerApi, getMCPServers, saveMCPServer} from "@/lib/api/mcpServers";
import {getAllTools, saveAllTools} from "@/lib/api/tools";
import {getSettings, updateSetting, updateSystemPrompt} from "@/lib/api/settings";
import {FrontendProvider, MCPServerRequest, MCPServerResponse, Model, ProviderRequest, Tool} from "@/lib/api/types";
import {useModelsContext} from "./useModelsContext";

interface SettingsData {
  providers: FrontendProvider[];
  mcpServers: MCPServerResponse[];
  tools: Tool[];
  settings: Record<string, string>;
  systemPrompt: string;
}

interface SettingsDataContext {
  data: SettingsData;
  // Models from global context
  models: Model[];
  loading: boolean;
  loaded: boolean;
  
  fetchAll: () => Promise<void>;
  
  // Providers
  addProvider: (data: ProviderRequest) => Promise<void>;
  updateProvider: (data: ProviderRequest) => Promise<void>;
  deleteProvider: (id: string) => Promise<void>;
  
  // MCP Servers
  addMCPServer: (data: MCPServerRequest) => Promise<void>;
  updateMCPServer: (data: MCPServerRequest) => Promise<void>;
  deleteMCPServer: (id: string) => Promise<void>;
  
  // Tools
  updateToolsLocal: (tools: Tool[]) => void;
  saveTools: (tools: Tool[]) => Promise<void>;
  
  // Models - delegates to global context
  updateModelsLocal: (models: Model[]) => void;
  saveModels: (models: Model[]) => Promise<void>;
  getModelsByProvider: (providerId: string) => Model[];
  
  // Settings
  updateSettingsLocal: (key: string, value: string) => void;
  saveSettings: () => Promise<void>;
}

const Context = createContext<SettingsDataContext | undefined>(undefined);

export const SettingsDataProvider = ({ children }: { children: ReactNode }) => {
  // Use global models context
  const { 
    models, 
    updateModelsLocal: globalUpdateModels, 
    saveModels: globalSaveModels,
    getModelsByProvider,
    refreshModels 
  } = useModelsContext();

  const [data, setData] = useState<SettingsData>({
    providers: [],
    mcpServers: [],
    tools: [],
    settings: {},
    systemPrompt: ""
  });
  
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const pendingSettings = useRef<Record<string, string>>({});

  const fetchAll = useCallback(async () => {
    if (loading) return;
    setLoading(true);
    
    try {
      const [providersRes, mcpRes, toolsRes, settingsRes] = await Promise.all([
        getProviders(),
        getMCPServers(),
        getAllTools(),
        getSettings()
      ]);
      
      const frontendProviders = providersRes.map((p) => backendToFrontendProvider(p));
      
      setData({
        providers: frontendProviders,
        mcpServers: mcpRes,
        tools: toolsRes.tools.map(t => ({
          ...t,
          is_enabled: t.is_enabled ?? true,
          require_approval: t.require_approval ?? false
        })),
        settings: settingsRes.settings,
        systemPrompt: settingsRes.settings.systemPrompt || ""
      });
      
      setLoaded(true);
    } finally {
      setLoading(false);
    }
  }, [loading]);

  // Providers
  const refreshProviders = useCallback(async () => {
    const providers = await getProviders();
    const frontendProviders = providers.map((p) => backendToFrontendProvider(p));
    setData(d => ({ ...d, providers: frontendProviders }));
  }, []);

  const addProvider = useCallback(async (providerData: ProviderRequest) => {
    await saveProvider(providerData);
    await refreshProviders();
    // Refresh global models since backend auto-fetches models for new provider
    await refreshModels();
  }, [refreshProviders, refreshModels]);

  const updateProvider = useCallback(async (providerData: ProviderRequest) => {
    await saveProvider(providerData);
    await refreshProviders();
  }, [refreshProviders]);

  const deleteProviderFn = useCallback(async (id: string) => {
    await deleteProvider(id);
    setData(d => ({ ...d, providers: d.providers.filter(p => p.id !== id) }));
    // Refresh models since deleted provider's models should be gone
    await refreshModels();
  }, [refreshModels]);

  // MCP Servers
  const addMCPServer = useCallback(async (serverData: MCPServerRequest) => {
    const newServer = await saveMCPServer(serverData);
    setData(d => ({ ...d, mcpServers: [...d.mcpServers, newServer] }));
  }, []);

  const updateMCPServer = useCallback(async (serverData: MCPServerRequest) => {
    const updated = await saveMCPServer(serverData);
    setData(d => ({
      ...d,
      mcpServers: d.mcpServers.map(s => s.id === updated.id ? updated : s)
    }));
  }, []);

  const deleteMCPServer = useCallback(async (id: string) => {
    await deleteMCPServerApi(id);
    setData(d => ({ ...d, mcpServers: d.mcpServers.filter(s => s.id !== id) }));
  }, []);

  // Tools
  const updateToolsLocal = useCallback((tools: Tool[]) => {
    setData(d => ({ ...d, tools }));
  }, []);

  const saveTools = useCallback(async (tools: Tool[]) => {
    await saveAllTools(tools);
  }, []);

  // Settings
  const updateSettingsLocal = useCallback((key: string, value: string) => {
    pendingSettings.current[key] = value;
    setData(d => ({
      ...d,
      settings: { ...d.settings, [key]: value },
      systemPrompt: key === "systemPrompt" ? value : d.systemPrompt
    }));
  }, []);

  const saveSettings = useCallback(async () => {
    const updates = Object.entries(pendingSettings.current);
    await Promise.all(
      updates.map(([key, value]) => 
        key === "systemPrompt" ? updateSystemPrompt(value) : updateSetting(key, value)
      )
    );
    pendingSettings.current = {};
  }, []);

  return (
    <Context.Provider value={{
      data,
      models,
      loading,
      loaded,
      fetchAll,
      addProvider,
      updateProvider,
      deleteProvider: deleteProviderFn,
      addMCPServer,
      updateMCPServer,
      deleteMCPServer,
      updateToolsLocal,
      saveTools,
      updateModelsLocal: globalUpdateModels,
      saveModels: globalSaveModels,
      getModelsByProvider,
      updateSettingsLocal,
      saveSettings
    }}>
      {children}
    </Context.Provider>
  );
};

export const useSettingsData = () => {
  const ctx = useContext(Context);
  if (!ctx) throw new Error("useSettingsData must be used within SettingsDataProvider");
  return ctx;
};
