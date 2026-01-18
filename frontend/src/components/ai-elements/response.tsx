"use client";

import { CodeBlock, CodeBlockCopyButton } from "./code-block.tsx";
import type { ComponentProps, HTMLAttributes } from "react";
import { memo } from "react";
import ReactMarkdown, { type Options } from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import { cn } from "@/lib/utils";
import "katex/dist/katex.min.css";
import hardenReactMarkdown from "harden-react-markdown";

/**
 * Remark plugin to detect citations: ([Link])
 * It marks the link with a data-citation attribute and removes surrounding parentheses.
 */
const remarkLinkCitations = () => (tree: any) => {
  const visit = (node: any, index?: number, parent?: any) => {
    // Look for links or linkReferences inside paragraphs
    if (node.type === "link" || node.type === "linkReference") {
      const siblings = parent?.children;
      if (!siblings || typeof index !== "number") return;
      
      const prev = siblings[index - 1];
      const next = siblings[index + 1];

      // Check if surrounded by parentheses
      if (
        prev &&
        prev.type === "text" &&
        prev.value.trim().endsWith("(") &&
        next &&
        next.type === "text" &&
        next.value.trim().startsWith(")")
      ) {
        // Mark the link node so the 'a' component knows it's a citation
        node.data = {
          ...node.data,
          hProperties: {
            ...node.data?.hProperties,
            "data-citation": "true",
          },
        };

        // Remove the parentheses from the surrounding text nodes
        prev.value = prev.value.replace(/\(\s*$/, "");
        next.value = next.value.replace(/^\s*\)/, "");
      }
    }

    if (node.children) {
      for (let i = 0; i < node.children.length; i++) {
        visit(node.children[i], i, node);
      }
    }
  };

  visit(tree);
};

/**
 * Parses markdown text and removes incomplete tokens to prevent partial rendering
 * of links, images, bold, and italic formatting during streaming.
 */
function parseIncompleteMarkdown(text: string): string {
  if (!text) {
    return text;
  }

  let result = text;

  // Handle incomplete links and images
  // Pattern: [...] or ![...] where the closing ] is missing
  const linkImagePattern = /(!?\[)([^\]]*?)$/;
  const linkMatch = result.match(linkImagePattern);
  if (linkMatch) {
    // If we have an unterminated [ or ![, remove it and everything after
    const startIndex = result.lastIndexOf(linkMatch[1]);
    result = result.substring(0, startIndex);
  }

  // Handle incomplete bold formatting (**)
  const boldPattern = /(\*\*)([^*]*?)$/;
  const boldMatch = result.match(boldPattern);
  if (boldMatch) {
    // Count the number of ** in the entire string
    const asteriskPairs = (result.match(/\*\*/g) || []).length;
    // If odd number of **, we have an incomplete bold - complete it
    if (asteriskPairs % 2 === 1) {
      result = `${result}**`;
    }
  }

  // Handle incomplete italic formatting (__)
  const italicPattern = /(__)([^_]*?)$/;
  const italicMatch = result.match(italicPattern);
  if (italicMatch) {
    // Count the number of __ in the entire string
    const underscorePairs = (result.match(/__/g) || []).length;
    // If odd number of __, we have an incomplete italic - complete it
    if (underscorePairs % 2 === 1) {
      result = `${result}__`;
    }
  }

  // Handle incomplete single asterisk italic (*)
  const singleAsteriskPattern = /(\*)([^*]*?)$/;
  const singleAsteriskMatch = result.match(singleAsteriskPattern);
  if (singleAsteriskMatch) {
    // Count single asterisks that aren't part of **
    const singleAsterisks = result.split("").reduce((acc, char, index) => {
      if (char === "*") {
        // Check if it's part of a ** pair
        const prevChar = result[index - 1];
        const nextChar = result[index + 1];
        if (prevChar !== "*" && nextChar !== "*") {
          return acc + 1;
        }
      }
      return acc;
    }, 0);

    // If odd number of single *, we have an incomplete italic - complete it
    if (singleAsterisks % 2 === 1) {
      result = `${result}*`;
    }
  }

  // Handle incomplete single underscore italic (_)
  const singleUnderscorePattern = /(_)([^_]*?)$/;
  const singleUnderscoreMatch = result.match(singleUnderscorePattern);
  if (singleUnderscoreMatch) {
    // Count single underscores that aren't part of __
    const singleUnderscores = result.split("").reduce((acc, char, index) => {
      if (char === "_") {
        // Check if it's part of a __ pair
        const prevChar = result[index - 1];
        const nextChar = result[index + 1];
        if (prevChar !== "_" && nextChar !== "_") {
          return acc + 1;
        }
      }
      return acc;
    }, 0);

    // If odd number of single _, we have an incomplete italic - complete it
    if (singleUnderscores % 2 === 1) {
      result = `${result}_`;
    }
  }

  // Handle incomplete inline code blocks (`) - but avoid code blocks (```)
  const inlineCodePattern = /(`)([^`]*?)$/;
  const inlineCodeMatch = result.match(inlineCodePattern);
  if (inlineCodeMatch) {
    // Check if we're dealing with a code block (triple backticks)
    const allTripleBackticks = (result.match(/```/g) || []).length;

    // If we have an odd number of ``` sequences, we're inside an incomplete code block
    // In this case, don't complete inline code
    const insideIncompleteCodeBlock = allTripleBackticks % 2 === 1;

    if (!insideIncompleteCodeBlock) {
      // Count the number of single backticks that are NOT part of triple backticks
      let singleBacktickCount = 0;
      for (let i = 0; i < result.length; i++) {
        if (result[i] === "`") {
          // Check if this backtick is part of a triple backtick sequence
          const isTripleStart = result.substring(i, i + 3) === "```";
          const isTripleMiddle =
            i > 0 && result.substring(i - 1, i + 2) === "```";
          const isTripleEnd = i > 1 && result.substring(i - 2, i + 1) === "```";

          if (!isTripleStart && !isTripleMiddle && !isTripleEnd) {
            singleBacktickCount++;
          }
        }
      }

      // If odd number of single backticks, we have an incomplete inline code - complete it
      if (singleBacktickCount % 2 === 1) {
        result = `${result}\``;
      }
    }
  }

  // Handle incomplete strikethrough formatting (~~)
  const strikethroughPattern = /(~~)([^~]*?)$/;
  const strikethroughMatch = result.match(strikethroughPattern);
  if (strikethroughMatch) {
    // Count the number of ~~ in the entire string
    const tildePairs = (result.match(/~~/g) || []).length;
    // If odd number of ~~, we have an incomplete strikethrough - complete it
    if (tildePairs % 2 === 1) {
      result = `${result}~~`;
    }
  }

  // // Remove parentheses from citations e.g. ([link]) -> [link]
  // result = result.replace(/\(\s*(\[[^\]]+\](?:\[[^\]]*\]|\([^\)]+\)))\s*\)/g, "$1");

  return result;
}

// Create a hardened version of ReactMarkdown
const HardenedMarkdown = hardenReactMarkdown(ReactMarkdown);

export type ResponseProps = HTMLAttributes<HTMLDivElement> & {
  options?: Options;
  children: Options["children"];
  allowedImagePrefixes?: ComponentProps<
    ReturnType<typeof hardenReactMarkdown>
  >["allowedImagePrefixes"];
  allowedLinkPrefixes?: ComponentProps<
    ReturnType<typeof hardenReactMarkdown>
  >["allowedLinkPrefixes"];
  defaultOrigin?: ComponentProps<
    ReturnType<typeof hardenReactMarkdown>
  >["defaultOrigin"];
  parseIncompleteMarkdown?: boolean;
};

const components: Options["components"] = {
  ol: ({ node, children, className, ...props }) => (
    <ol
      className={cn("pl-2 ml-4 list-outside list-decimal my-3 space-y-2", className)}
      {...props}
    >
      {children}
    </ol>
  ),
  li: ({ node, children, className, ...props }) => (
    <li className={cn("pl-2 leading-relaxed", className)} {...props}>
      {children}
    </li>
  ),
  ul: ({ node, children, className, ...props }) => (
    <ul
      className={cn("pl-4 ml-4 list-outside list-disc my-3 space-y-2", className)}
      {...props}
    >
      {children}
    </ul>
  ),
  strong: ({ node, children, className, ...props }) => (
    <span className={cn("font-semibold", className)} {...props}>
      {children}
    </span>
  ),

  a: ({ node, children, className, ...props }) => {
    // Check if the link was marked as a citation by the remark plugin
    const isCitation = (props as any)["data-citation"] === "true";

    return (
      <a
        className={cn(
          "transition-colors ease-in-out !duration-100",
          isCitation
            ? "inline-flex items-center rounded-full bg-secondary px-2 py-0.5 text-[0.5625rem] leading-[0.8rem] text-secondary-foreground no-underline hover:bg-secondary-foreground/90 hover:text-primary-foreground"
            : "text-primary underline underline-offset-4 hover:text-primary/80",
          className
        )}
        rel="noreferrer"
        target="_blank"
        {...props}
      >
        {children}
      </a>
    );
  },

  h1: ({ node, children, className, ...props }) => (
    <h1
      className={cn(
        "mt-8 mb-4 font-semibold text-3xl leading-tight",
        className,
      )}
      {...props}
    >
      {children}
    </h1>
  ),
  h2: ({ node, children, className, ...props }) => (
    <h2
      className={cn(
        "mt-8 mb-4 font-semibold text-2xl leading-tight",
        className,
      )}
      {...props}
    >
      {children}
    </h2>
  ),
  h3: ({ node, children, className, ...props }) => (
    <h3
      className={cn("mt-6 mb-3 font-semibold text-xl leading-tight", className)}
      {...props}
    >
      {children}
    </h3>
  ),
  h4: ({ node, children, className, ...props }) => (
    <h4
      className={cn("mt-6 mb-3 font-semibold text-lg leading-tight", className)}
      {...props}
    >
      {children}
    </h4>
  ),
  h5: ({ node, children, className, ...props }) => (
    <h5
      className={cn(
        "mt-6 mb-3 font-semibold text-base leading-tight",
        className,
      )}
      {...props}
    >
      {children}
    </h5>
  ),
  h6: ({ node, children, className, ...props }) => (
    <h6
      className={cn("mt-6 mb-3 font-semibold text-sm leading-tight", className)}
      {...props}
    >
      {children}
    </h6>
  ),
  p: ({ node, children, className, ...props }) => (
    <p
      className={cn("mb-4 break-words [line-height:1.75rem]", className)}
      {...props}
    >
      {children}
    </p>
  ),
  pre: ({ node, className, children }) => {
    let language = "javascript";

    const childrenIsCode =
      typeof children === "object" &&
      children !== null &&
      "type" in children &&
      children.type === "code";

    if (childrenIsCode && "props" in children) {
      const childProps = children.props as { className?: string };
      if (typeof childProps.className === "string") {
        language = childProps.className.replace("language-", "");
      }
    } else if (typeof node?.properties?.className === "string") {
      language = node.properties.className.replace("language-", "");
    }

    if (!childrenIsCode) {
      return (
        <pre className="overflow-x-auto whitespace-pre-wrap break-words">
          {children}
        </pre>
      );
    }

    return (
      <CodeBlock
        className={cn("my-4 h-auto w-full overflow-hidden", className)}
        code={(children.props as { children: string }).children}
        language={language}
      >
        <CodeBlockCopyButton
          onCopy={() => console.log("Copied code to clipboard")}
          onError={() => console.error("Failed to copy code to clipboard")}
        />
      </CodeBlock>
    );
  },
};

export const Response = memo(
  ({
    className,
    options,
    children,
    allowedImagePrefixes,
    allowedLinkPrefixes,
    defaultOrigin,
    parseIncompleteMarkdown: shouldParseIncompleteMarkdown = true,
    ...props
  }: ResponseProps) => {
    // Parse the children to remove incomplete markdown tokens if enabled
    const parsedChildren =
      typeof children === "string" && shouldParseIncompleteMarkdown
        ? parseIncompleteMarkdown(children)
        : children;

    return (
      <div
        className={cn(
          "size-full w-full max-w-full overflow-hidden [&>*:first-child]:mt-0 [&>*:last-child]:mb-0 [&_p]:mb-4 [&_p]:[line-height:1.75rem] [&_p]:break-words prose-p:mb-4 prose-p:[line-height:1.75rem] prose-p:break-words [&_table]:table-auto [&_table]:border-collapse [&_table]:w-full [&_table]:my-6 [&_table_th]:border [&_table_td]:border [&_table_td]:px-3 [&_table_td]:py-2 [&_table_th]:px-3 [&_table_th]:py-2 [&_hr]:mb-8 [&_hr]:mt-6",
          className,
        )}
        {...props}
      >
        <HardenedMarkdown
          components={components}
          rehypePlugins={[rehypeKatex]}
          remarkPlugins={[remarkGfm, remarkMath, remarkLinkCitations]}
          allowedImagePrefixes={allowedImagePrefixes ?? ["*"]}
          allowedLinkPrefixes={allowedLinkPrefixes ?? ["*"]}
          defaultOrigin={defaultOrigin}
          {...options}
        >
          {parsedChildren}
        </HardenedMarkdown>
      </div>
    );
  },
  (prevProps, nextProps) => prevProps.children === nextProps.children,
);
