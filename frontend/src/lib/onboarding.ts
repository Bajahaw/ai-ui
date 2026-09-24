export type OnboardingStep = "provider" | "mcp" | null;

export const MCP_ONBOARDING_SKIPPED_KEY = "ai-ui:onboarding:mcp-skipped";

/** Built-in servers (id starts with "default") don't count as user-added. */
export function isDefaultMCPServer(id: string): boolean {
  return id.startsWith("default");
}

export function getOnboardingStep(input: {
  loaded: boolean;
  providerCount: number;
  customMCPServerCount: number;
  totalMessages: number;
  mcpSkipped: boolean;
}): OnboardingStep {
  if (!input.loaded) return null;
  if (input.providerCount === 0) return "provider";
  if (
    input.customMCPServerCount === 0 &&
    input.totalMessages === 0 &&
    !input.mcpSkipped
  ) {
    return "mcp";
  }
  return null;
}
