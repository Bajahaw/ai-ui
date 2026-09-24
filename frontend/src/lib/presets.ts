import {
  SiAnthropic,
  SiGithub,
  SiGooglegemini,
  SiLinear,
  SiLmstudio,
  SiNotion,
  SiOllama,
  SiOpenrouter,
  SiStripe,
} from "@icons-pack/react-simple-icons";
import { Blocks, Flame, Search } from "lucide-react";
import { ChatGPTIcon, type BrandIcon } from "@/components/icons/brand-icons";

export interface ProviderPreset {
  id: string;
  name: string;
  icon: BrandIcon;
  /** OAuth presets sign in instead of opening the API form. */
  oauth?: boolean;
  baseUrl?: string;
}

export interface MCPPreset {
  id: string;
  name: string;
  icon: BrandIcon;
  endpoint?: string;
  /** Header keys the service expects; values are left for the user to fill. */
  headers?: Record<string, string>;
}

export const PROVIDER_PRESETS: ProviderPreset[] = [
  { id: "chatgpt", name: "ChatGPT", icon: ChatGPTIcon, oauth: true },
  {
    id: "openai",
    name: "OpenAI",
    icon: ChatGPTIcon,
    baseUrl: "https://api.openai.com/v1",
  },
  {
    id: "anthropic",
    name: "Anthropic",
    icon: SiAnthropic,
    baseUrl: "https://api.anthropic.com/v1",
  },
  {
    id: "openrouter",
    name: "OpenRouter",
    icon: SiOpenrouter,
    baseUrl: "https://openrouter.ai/api/v1",
  },
  {
    id: "gemini",
    name: "Gemini",
    icon: SiGooglegemini,
    baseUrl: "https://generativelanguage.googleapis.com/v1beta/openai",
  },
  {
    id: "ollama",
    name: "Ollama",
    icon: SiOllama,
    baseUrl: "http://localhost:11434/v1",
  },
  {
    id: "lmstudio",
    name: "LM Studio",
    icon: SiLmstudio,
    baseUrl: "http://localhost:1234/v1",
  },
];

export const MCP_PRESETS: MCPPreset[] = [
  {
    id: "github",
    name: "GitHub",
    icon: SiGithub,
    endpoint: "https://api.githubcopilot.com/mcp/",
  },
  {
    id: "notion",
    name: "Notion",
    icon: SiNotion,
    endpoint: "https://mcp.notion.com/mcp",
  },
  {
    id: "linear",
    name: "Linear",
    icon: SiLinear,
    endpoint: "https://mcp.linear.app/mcp",
  },
  {
    id: "composio",
    name: "Composio",
    icon: Blocks,
    endpoint: "https://connect.composio.dev/mcp",
    headers: { "x-consumer-api-key": "" },
  },
  {
    id: "firecrawl",
    name: "Firecrawl",
    icon: Flame,
    endpoint: "https://mcp.firecrawl.dev/v2/mcp",
  },
  {
    id: "exa",
    name: "Exa",
    icon: Search,
    endpoint: "https://mcp.exa.ai/mcp",
    headers: { "x-api-key": "" },
  },
  {
    id: "stripe",
    name: "Stripe",
    icon: SiStripe,
    endpoint: "https://mcp.stripe.com",
  },
];
