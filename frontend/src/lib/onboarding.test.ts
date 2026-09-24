import { describe, expect, it } from "vitest";
import { getOnboardingStep, isDefaultMCPServer } from "./onboarding";

const base = {
  loaded: true,
  providerCount: 0,
  customMCPServerCount: 0,
  totalMessages: 0,
  mcpSkipped: false,
};

describe("getOnboardingStep", () => {
  it("shows nothing until settings have loaded", () => {
    expect(getOnboardingStep({ ...base, loaded: false })).toBeNull();
  });

  it("asks for a provider first", () => {
    expect(getOnboardingStep(base)).toBe("provider");
    expect(
      getOnboardingStep({ ...base, customMCPServerCount: 2, totalMessages: 9 }),
    ).toBe("provider");
  });

  it("suggests MCP once a provider exists and nothing has been sent", () => {
    expect(getOnboardingStep({ ...base, providerCount: 1 })).toBe("mcp");
  });

  it("hides the MCP step after the first message, a custom server, or a skip", () => {
    expect(
      getOnboardingStep({ ...base, providerCount: 1, totalMessages: 1 }),
    ).toBeNull();
    expect(
      getOnboardingStep({ ...base, providerCount: 1, customMCPServerCount: 1 }),
    ).toBeNull();
    expect(
      getOnboardingStep({ ...base, providerCount: 1, mcpSkipped: true }),
    ).toBeNull();
  });
});

describe("isDefaultMCPServer", () => {
  it("treats built-in default-* ids as default", () => {
    expect(isDefaultMCPServer("default-alice")).toBe(true);
    expect(isDefaultMCPServer("github-1a2b")).toBe(false);
  });
});
