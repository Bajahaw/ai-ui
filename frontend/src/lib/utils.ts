import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function createSearchMatcher(search: string) {
  const terms = search.trim().toLowerCase().split(/\s+/).filter(Boolean);
  return (...fields: (string | null | undefined)[]) => {
    const values = fields.map((field) => field?.toLowerCase() ?? "");
    return terms.every((term) => values.some((value) => value.includes(term)));
  };
}
