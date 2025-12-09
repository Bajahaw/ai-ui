import { useState, useEffect, useCallback } from "react";
import {
    getSettings,
    updateSettings,
    updateSystemPrompt,
    updateSetting,
} from "@/lib/api/settings";
import { Settings } from "@/lib/api/types";

interface UseSettingsReturn {
    settings: Record<string, string>;
    systemPrompt: string;
    isLoading: boolean;
    error: string | null;
    refreshSettings: () => Promise<void>;
    updateAllSettings: (newSettings: Record<string, string>) => Promise<void>;
    updateSystemPromptSetting: (prompt: string) => Promise<void>;
    updateSingleSetting: (key: string, value: string) => Promise<void>;
    getSingleSetting: (key: string) => string;
    clearError: () => void;
}

export const useSettings = (): UseSettingsReturn => {
    const [settings, setSettings] = useState<Record<string, string>>({});
    const [systemPrompt, setSystemPrompt] = useState<string>("");
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const clearError = useCallback(() => {
        setError(null);
    }, []);

    const refreshSettings = useCallback(async () => {
        try {
            setIsLoading(true);
            setError(null);

            const settingsResponse = await getSettings();
            setSettings(settingsResponse.settings);
            setSystemPrompt(settingsResponse.settings.systemPrompt || "");
        } catch (err) {
            const errorMessage =
                err instanceof Error ? err.message : "Failed to load settings";
            setError(errorMessage);
            console.error("Error loading settings:", err);
        } finally {
            setIsLoading(false);
        }
    }, []);

    const updateAllSettings = useCallback(
        async (newSettings: Record<string, string>) => {
            try {
                setError(null);
                const settingsPayload: Settings = {
                    settings: newSettings,
                };

                await updateSettings(settingsPayload);

                // Update local state
                setSettings(newSettings);
                setSystemPrompt(newSettings.systemPrompt || "");
            } catch (err) {
                const errorMessage =
                    err instanceof Error ? err.message : "Failed to update settings";
                setError(errorMessage);
                throw err;
            }
        },
        [],
    );

    const updateSystemPromptSetting = useCallback(async (prompt: string) => {
        try {
            setError(null);
            await updateSystemPrompt(prompt);

            // Update local state
            setSystemPrompt(prompt);
            setSettings((prev) => ({ ...prev, systemPrompt: prompt }));
        } catch (err) {
            const errorMessage =
                err instanceof Error ? err.message : "Failed to update system prompt";
            setError(errorMessage);
            throw err;
        }
    }, []);

    const updateSingleSetting = useCallback(
        async (key: string, value: string) => {
            try {
                setError(null);
                await updateSetting(key, value);

                // Update local state
                setSettings((prev) => ({ ...prev, [key]: value }));

                // Update system prompt if that's what was changed
                if (key === "systemPrompt") {
                    setSystemPrompt(value);
                }
            } catch (err) {
                const errorMessage =
                    err instanceof Error ? err.message : `Failed to update ${key}`;
                setError(errorMessage);
                throw err;
            }
        },
        [],
    );

    const getSingleSetting = useCallback(
        (key: string): string => {
            return settings[key] || "";
        },
        [settings],
    );

    // Load settings on mount
    useEffect(() => {
        refreshSettings();
    }, [refreshSettings]);

    return {
        settings,
        systemPrompt,
        isLoading,
        error,
        refreshSettings,
        updateAllSettings,
        updateSystemPromptSetting,
        updateSingleSetting,
        getSingleSetting,
        clearError,
    };
};
