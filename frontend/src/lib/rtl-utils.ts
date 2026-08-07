/**
 * RTL text direction utilities for Arabic content
 */

import {
  Children,
  createElement,
  isValidElement,
  type ReactNode,
} from "react";
import { cn } from "@/lib/utils";

// Unicode ranges for Arabic script
const ARABIC_UNICODE_RANGES = [
  [0x0600, 0x06ff], // Arabic
  [0x0750, 0x077f], // Arabic Supplement
  [0xfb50, 0xfdff], // Arabic Presentation Forms-A
  [0xfe70, 0xfeff], // Arabic Presentation Forms-B
];

const BLOCK_TAGS = new Set([
  "p",
  "div",
  "ul",
  "ol",
  "li",
  "pre",
  "blockquote",
  "table",
  "thead",
  "tbody",
  "tr",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "hr",
  "section",
  "article",
]);

/**
 * Check if a character is Arabic
 */
function isArabicCharacter(char: string): boolean {
  const charCode = char.charCodeAt(0);
  return ARABIC_UNICODE_RANGES.some(
    ([start, end]) => charCode >= start && charCode <= end,
  );
}

/**
 * Check if a character is a strong LTR letter (Latin, etc.)
 */
function isLtrLetter(char: string): boolean {
  return /[A-Za-z\u00C0-\u024F]/.test(char);
}

/**
 * Whether text should be RTL based on the first strong letter (ties / empty → LTR).
 * Digits, punctuation, and whitespace are skipped until a letter is found.
 */
export function isPrimarilyRtl(text: string): boolean {
  if (!text) return false;

  for (const char of text) {
    if (isArabicCharacter(char)) return true;
    if (isLtrLetter(char)) return false;
  }

  return false;
}

/**
 * Simple check if text contains Arabic content
 */
export function containsArabic(text: string): boolean {
  if (!text || text.length === 0) return false;
  return Array.from(text).some(isArabicCharacter);
}

/**
 * Get text alignment class for content
 */
export function getTextAlignment(text: string): string {
  return isPrimarilyRtl(text) ? "text-right" : "text-left";
}

/**
 * Get text direction class for content
 */
export function getTextDirection(text: string): string {
  return isPrimarilyRtl(text) ? "rtl" : "ltr";
}

/**
 * Extract plain text from React children
 */
export function getChildrenText(children: ReactNode): string {
  return Children.toArray(children)
    .map((child) => {
      if (typeof child === "string" || typeof child === "number") {
        return String(child);
      }
      if (isValidElement(child)) {
        return getChildrenText(
          (child.props as { children?: ReactNode }).children,
        );
      }
      return "";
    })
    .join("");
}

/**
 * Extract plain text from a hast/mdast node (react-markdown `node` prop)
 */
export function getNodeText(node: unknown): string {
  if (!node || typeof node !== "object") return "";

  const n = node as { type?: string; value?: string; children?: unknown[] };

  if (n.type === "text" && typeof n.value === "string") {
    return n.value;
  }

  if (Array.isArray(n.children)) {
    return n.children.map(getNodeText).join("");
  }

  return "";
}

/**
 * Direction + alignment classes for plain text
 */
export function getDirectionClasses(text: string): string {
  return cn(getTextDirection(text), getTextAlignment(text));
}

/**
 * Direction + alignment classes for a markdown block node
 */
export function getBlockDirectionClasses(node: unknown): string {
  return getDirectionClasses(getNodeText(node));
}

function isBrElement(child: ReactNode): boolean {
  return isValidElement(child) && child.type === "br";
}

function isBlockElement(child: ReactNode): boolean {
  if (!isValidElement(child)) return false;
  const type = child.type;
  return typeof type === "string" && BLOCK_TAGS.has(type);
}

export type RenderDirectionalLinesOptions = {
  /**
   * Keep content on the same line as a list-inside marker
   * (block wrappers alone would push text under the bullet).
   */
  besideMarker?: boolean;
};

/**
 * Split inline markdown children on <br>/newlines and wrap each line
 * with its own direction/alignment. Block children are left untouched.
 */
export function renderDirectionalLines(
  children: ReactNode,
  options: RenderDirectionalLinesOptions = {},
): ReactNode {
  const { besideMarker = false } = options;
  const items = Children.toArray(children);

  if (items.length === 0) return children;

  // Nested blocks (e.g. <li><p>...) handle their own direction
  if (items.some(isBlockElement)) {
    return children;
  }

  const lines: ReactNode[][] = [[]];

  const pushTextParts = (text: string) => {
    const parts = text.split("\n");
    parts.forEach((part, index) => {
      if (part) {
        lines[lines.length - 1].push(part);
      }
      if (index < parts.length - 1) {
        lines.push([]);
      }
    });
  };

  for (const child of items) {
    if (isBrElement(child)) {
      lines.push([]);
      continue;
    }

    if (typeof child === "string") {
      pushTextParts(child);
      continue;
    }

    lines[lines.length - 1].push(child);
  }

  const renderedLines =
    lines.length === 1
      ? (() => {
          const text = getChildrenText(lines[0]);
          if (!text.trim()) return null;
          return createElement(
            "span",
            { className: cn("block w-full", getDirectionClasses(text)) },
            ...lines[0],
          );
        })()
      : lines.map((line, index) => {
          if (line.length === 0) {
            return createElement("span", {
              key: `rtl-line-empty-${index}`,
              className: "block w-full",
            });
          }

          const text = getChildrenText(line);
          return createElement(
            "span",
            {
              key: `rtl-line-${index}`,
              className: cn("block w-full", getDirectionClasses(text)),
            },
            ...line,
          );
        });

  if (renderedLines === null) return children;

  // list-inside marker is inline; wrap blocks so they sit beside it.
  // Use full width (not shrink-to-fit) so LTR lines start at the left edge
  // even when the parent <li> is RTL.
  if (besideMarker) {
    return createElement(
      "span",
      { className: "inline-block w-[calc(100%-1.5em)] align-top" },
      ...(Array.isArray(renderedLines) ? renderedLines : [renderedLines]),
    );
  }

  return renderedLines;
}

/**
 * Convenience: directional classes from React children text
 */
export function getChildrenDirectionClasses(children: ReactNode): string {
  return getDirectionClasses(getChildrenText(children));
}
