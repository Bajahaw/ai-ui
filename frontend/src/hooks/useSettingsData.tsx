import {createContext, ReactNode, useCallback, useContext, useRef, useState} from "react";
import {
  backendToFrontendProvider,
  deleteProvider,
  getProviderModels,
  getProviders,
  saveProvider
} from "@/lib/api/providers";
import {deleteMCPServer as deleteMCPServerApi, getMCPServers, saveMCPServer} from "@/lib/api/mcpServers";
import {getAllTools, saveAllTools} from "@/lib/api/tools";
import {getSettings, updateSetting, updateSystemPrompt} from "@/lib/api/settings";
import {getAllModels, updateModelEnableFlags} from "@/lib/api/models";
import {FrontendProvider, MCPServerRequest, MCPServerResponse, Model, ProviderRequest, Tool} from "@/lib/api/types";

interface SettingsData {
  providers: FrontendProvider[];
  mcpServers: MCPServerResponse[];
  tools: Tool[];
  models: Model[];
  settings: Record<string, string>;
  systemPrompt: string;
}

interface SettingsDataContext {
  data: SettingsData;
  loading: boolean;
  loaded: boolean;
  
  fetchAll: () => Promise<void>;
  
  // Providers
  addProvider: (data: ProviderRequest) => Promise<void>;
  updateProvider: (data: ProviderRequest) => Promise<void>;
  deleteProvider: (id: string) => Promise<void>;
  loadProviderModels: (id: string) => Promise<void>;
  
  // MCP Servers
  addMCPServer: (data: MCPServerRequest) => Promise<void>;
  updateMCPServer: (data: MCPServerRequest) => Promise<void>;
  deleteMCPServer: (id: string) => Promise<void>;
  
  // Tools
  updateToolsLocal: (tools: Tool[]) => void;
  saveTools: (tools: Tool[]) => Promise<void>;
  
  // Models
  updateModelsLocal: (models: Model[]) => void;
  saveModels: (models: Model[]) => Promise<void>;
  
  // Settings
  updateSettingsLocal: (key: string, value: string) => void;
  saveSettings: () => Promise<void>;
}

const Context = createContext<SettingsDataContext | undefined>(undefined);

export const SettingsDataProvider = ({ children }: { children: ReactNode }) => {
  const [data, setData] = useState<SettingsData>({
    providers: [],
    mcpServers: [],
    tools: [],
    models: [],
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
      const [providersRes, mcpRes, toolsRes, modelsRes, settingsRes] = await Promise.all([
        getProviders(),
        getMCPServers(),
        getAllTools(),
        getAllModels(),
        getSettings()
      ]);
      
      const providersWithModels = await Promise.all(
        providersRes.map(async (p) => {
          try {
            const models = await getProviderModels(p.id);
            return backendToFrontendProvider(p, models);
          } catch {
            return backendToFrontendProvider(p);
          }
        })
      );
      
      setData({
        providers: providersWithModels,
        mcpServers: mcpRes,
        tools: toolsRes.tools.map(t => ({
          ...t,
          is_enabled: t.is_enabled ?? true,
          require_approval: t.require_approval ?? false
        })),
        models: modelsRes.models,
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
    const providersWithModels = await Promise.all(
      providers.map(async (p) => {
        try {
          const models = await getProviderModels(p.id);
          return backendToFrontendProvider(p, models);
        } catch {
          return backendToFrontendProvider(p);
        }
      })
    );
    setData(d => ({ ...d, providers: providersWithModels }));
  }, []);

  const addProvider = useCallback(async (providerData: ProviderRequest) => {
    await saveProvider(providerData);
    await refreshProviders();
  }, [refreshProviders]);

  const updateProvider = useCallback(async (providerData: ProviderRequest) => {
    await saveProvider(providerData);
    await refreshProviders();
  }, [refreshProviders]);

  const deleteProviderFn = useCallback(async (id: string) => {
    await deleteProvider(id);
    setData(d => ({ ...d, providers: d.providers.filter(p => p.id !== id) }));
  }, []);

  const loadProviderModels = useCallback(async (id: string) => {
    const models = await getProviderModels(id);
    setData(d => ({
      ...d,
      providers: d.providers.map(p => 
        p.id === id ? { ...p, models: models.models } : p
      )
    }));
  }, []);

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

  // Models
  const updateModelsLocal = useCallback((models: Model[]) => {
    setData(d => ({ ...d, models }));
  }, []);

  const saveModels = useCallback(async (models: Model[]) => {
    await updateModelEnableFlags(models);
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
      loading,
      loaded,
      fetchAll,
      addProvider,
      updateProvider,
      deleteProvider: deleteProviderFn,
      loadProviderModels,
      addMCPServer,
      updateMCPServer,
      deleteMCPServer,
      updateToolsLocal,
      saveTools,
      updateModelsLocal,
      saveModels,
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
