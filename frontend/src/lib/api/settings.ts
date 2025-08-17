import { getApiUrl } from "@/lib/config";
import { Settings } from "./types";

// Get all settings
export const getSettings = async (): Promise<Settings> => {
  const response = await fetch(getApiUrl("/api/settings/"), {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch settings: ${response.statusText}`);
  }

  return response.json();
};

// Update settings
export const updateSettings = async (settings: Settings): Promise<void> => {
  const response = await fetch(getApiUrl("/api/settings/update"), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(settings),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to update settings: ${response.statusText}`);
  }
};

// Helper functions for specific settings
export const getSystemPrompt = async (): Promise<string> => {
  try {
    const settings = await getSettings();
    return settings.settings.systemPrompt || "";
  } catch (error) {
    console.error("Failed to get system prompt:", error);
    return "";
  }
};

export const updateSystemPrompt = async (
  systemPrompt: string,
): Promise<void> => {
  const settings: Settings = {
    settings: {
      systemPrompt,
    },
  };
  await updateSettings(settings);
};

// Get a specific setting by key
export const getSetting = async (key: string): Promise<string> => {
  try {
    const settings = await getSettings();
    return settings.settings[key] || "";
  } catch (error) {
    console.error(`Failed to get setting ${key}:`, error);
    return "";
  }
};

// Update a specific setting by key
export const updateSetting = async (
  key: string,
  value: string,
): Promise<void> => {
  try {
    // Get current settings first to preserve other values
    const currentSettings = await getSettings();
    const updatedSettings: Settings = {
      settings: {
        ...currentSettings.settings,
        [key]: value,
      },
    };
    await updateSettings(updatedSettings);
  } catch (error) {
    console.error(`Failed to update setting ${key}:`, error);
    throw error;
  }
};
