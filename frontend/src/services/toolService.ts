import { api } from '@/services/api';

export interface ToolItem {
  name: string;
  display_name: string;
  description: string;
  icon: string;
  category: string;
  keywords: string[];
  source: string;
}

export interface ToolCategory {
  name: string;
  count: number;
  tools: ToolItem[];
}

export interface ToolsResponse {
  categories: ToolCategory[];
  total: number;
}

/**
 * Fetch all tools from the backend, grouped by category.
 * Uses GET /api/tools (requires authentication).
 */
export async function fetchTools(): Promise<ToolsResponse> {
  const data = await api.get<ToolsResponse>('/api/tools');
  return data;
}
