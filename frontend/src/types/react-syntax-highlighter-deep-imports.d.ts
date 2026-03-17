declare module "react-syntax-highlighter/dist/esm/prism-async-light" {
  import type { ComponentType } from "react";
  import type { SyntaxHighlighterProps } from "react-syntax-highlighter";

  type PrismAsyncLightComponent = ComponentType<SyntaxHighlighterProps> & {
    supportedLanguages?: string[];
    registerLanguage?: (name: string, syntax: unknown) => void;
  };

  const PrismAsyncLight: PrismAsyncLightComponent;
  export default PrismAsyncLight;
}

declare module "react-syntax-highlighter/dist/esm/styles/prism" {
  import type { CSSProperties } from "react";

  type PrismStyle = { [key: string]: CSSProperties };

  export const oneDark: PrismStyle;
  export const oneLight: PrismStyle;
}
