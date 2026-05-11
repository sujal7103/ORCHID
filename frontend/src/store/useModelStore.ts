import { create } from 'zustand';
import { devtools, persist } from 'zustand/middleware';
import { fetchModels as fetchModelsAPI } from '@/services/modelService';
import type { Model } from '@/types/websocket';

// Helper function to get user-specific storage name
function getUserStorageName(baseName: string): string {
  try {
    const authStorage = localStorage.getItem('auth-storage');
    if (authStorage) {
      const { state } = JSON.parse(authStorage);
      if (state?.user?.id) {
        return `${baseName}-${state.user.id}`;
      }
    }
  } catch (error) {
    console.warn('Failed to get user ID for storage name:', error);
  }
  return baseName;
}

export type ModelContext = 'startBlock' | 'builderMode' | 'askMode' | 'llmBlocks';

interface ModelState {
  // State
  models: Model[];
  selectedModelId: string | null;
  isLoading: boolean;
  error: string | null;

  // Context-specific defaults (global, not per-agent)
  // Tracks which models users select most frequently for different purposes
  defaultModels: {
    startBlock: string | null; // For variable/input blocks
    builderMode: string | null; // For workflow generation
    askMode: string | null; // For Ask mode chat
    llmBlocks: string | null; // For LLM inference blocks
  };

  // Actions
  fetchModels: (requireAuth?: boolean) => Promise<void>;
  setSelectedModel: (modelId: string) => void;
  getSelectedModel: () => Model | null;
  clearError: () => void;

  // Context-specific model selection
  setDefaultModelForContext: (context: ModelContext, modelId: string) => void;
  getDefaultModelForContext: (context: ModelContext) => string | null;
  getDefaultModelObjectForContext: (context: ModelContext) => Model | null;
}

export const useModelStore = create<ModelState>()(
  devtools(
    persist(
      (set, get) => ({
        // Initial state
        models: [],
        selectedModelId: null,
        isLoading: false,
        error: null,
        defaultModels: {
          startBlock: null,
          builderMode: null,
          askMode: null,
          llmBlocks: null,
        },

        // Actions
        fetchModels: async (requireAuth = true) => {
          set({ isLoading: true, error: null });

          try {
            const models = await fetchModelsAPI(requireAuth);

            set({ models, isLoading: false });

            // Auto-select first model if none selected
            const { selectedModelId, defaultModels } = get();
            if (!selectedModelId && models.length > 0) {
              // If there's a defaultModel for builderMode, use that, otherwise first model
              const defaultBuilderId = defaultModels.builderMode;
              const defaultModel = defaultBuilderId
                ? models.find(m => m.id === defaultBuilderId)
                : null;
              set({ selectedModelId: defaultModel?.id || models[0].id });
            }
          } catch (error) {
            const errorMessage = error instanceof Error ? error.message : 'Failed to fetch models';
            set({ error: errorMessage, isLoading: false });
            console.error('Error fetching models:', error);
          }
        },

        setSelectedModel: modelId => {
          set({ selectedModelId: modelId });
        },

        getSelectedModel: () => {
          const { models, selectedModelId } = get();
          return models.find(m => m.id === selectedModelId) || null;
        },

        clearError: () => {
          set({ error: null });
        },

        // Context-specific model selection
        setDefaultModelForContext: (context, modelId) => {
          set(state => ({
            defaultModels: {
              ...state.defaultModels,
              [context]: modelId,
            },
          }));
        },

        getDefaultModelForContext: context => {
          const { defaultModels } = get();
          return defaultModels[context];
        },

        getDefaultModelObjectForContext: context => {
          const { defaultModels, models } = get();
          const modelId = defaultModels[context];
          if (!modelId) return null;
          return models.find(m => m.id === modelId) || null;
        },
      }),
      {
        name: 'model-storage',
        partialize: state => ({
          selectedModelId: state.selectedModelId,
          defaultModels: state.defaultModels,
        }),
        // Use a custom storage implementation that uses user-specific keys
        storage: {
          getItem: (name: string) => {
            const userSpecificName = getUserStorageName(name);
            const str = localStorage.getItem(userSpecificName);
            if (!str) return null;
            return JSON.parse(str);
          },
          setItem: (name: string, value: unknown) => {
            const userSpecificName = getUserStorageName(name);
            localStorage.setItem(userSpecificName, JSON.stringify(value));
          },
          removeItem: (name: string) => {
            const userSpecificName = getUserStorageName(name);
            localStorage.removeItem(userSpecificName);
          },
        },
      }
    ),
    { name: 'ModelStore' }
  )
);
