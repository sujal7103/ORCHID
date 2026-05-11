/**
 * Normalizes a block name to a consistent ID format for variable interpolation.
 *
 * Rules:
 * - Converts to lowercase
 * - Replaces non-alphanumeric characters with hyphens
 * - Trims leading/trailing hyphens
 *
 * Examples:
 * - "Search Latest News" → "search-latest-news"
 * - "Send to Discord!!!" → "send-to-discord"
 * - "Get Current Time" → "get-current-time"
 * - "get_current_time" → "get-current-time"
 */
export function normalizeBlockName(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-') // Replace non-alphanumeric with hyphen
    .replace(/^-+|-+$/g, ''); // Trim hyphens from start and end
}
