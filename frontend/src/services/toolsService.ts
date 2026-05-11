// Re-export from the singular toolService (the actual implementation)
// This file exists because some components import from 'toolsService' (plural)
import { api } from './api';

export interface Tool {
  name: string;
  display_name: string;
  description: string;
  icon: string;
  category: string;
  keywords: string[];
  source: string;
  parameters?: Record<string, unknown>;
}

export interface ToolCategory {
  name: string;
  count: number;
  tools: Tool[];
}

export interface ToolRecommendation {
  name: string;
  display_name: string;
  description: string;
  icon: string;
  category: string;
  keywords: string[];
  source: string;
  score: number;
  reason: string;
}

export async function fetchTools(): Promise<ToolCategory[]> {
  const response = await api.get<{ categories: ToolCategory[] }>('/api/tools');
  return response.categories || [];
}

export async function getToolRecommendations(
  blockName: string,
  blockDescription: string,
  blockType: string,
): Promise<ToolRecommendation[]> {
  const response = await api.post<{ recommendations: ToolRecommendation[] }>('/api/tools/recommend', {
    block_name: blockName,
    block_description: blockDescription,
    block_type: blockType,
  });
  return response.recommendations || [];
}
