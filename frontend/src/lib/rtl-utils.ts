/**
 * Simple RTL text alignment utility for Arabic content
 */

// Unicode ranges for Arabic script
const ARABIC_UNICODE_RANGES = [
  [0x0600, 0x06ff], // Arabic
  [0x0750, 0x077f], // Arabic Supplement
  [0xfb50, 0xfdff], // Arabic Presentation Forms-A
  [0xfe70, 0xfeff], // Arabic Presentation Forms-B
];

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
 * Simple check if text contains Arabic content
 */
export function containsArabic(text: string): boolean {
  if (!text || text.length === 0) return false;

  // Check if any character is Arabic
  return Array.from(text).some(isArabicCharacter);
}

/**
 * Get text alignment class for content
 */
export function getTextAlignment(text: string): string {
  return containsArabic(text) ? "text-right" : "text-left";
}
