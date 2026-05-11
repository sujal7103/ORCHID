import { create } from 'zustand';
import { devtools } from 'zustand/middleware';
import type {
  Credential,
  IntegrationCategory,
  CreateCredentialRequest,
  UpdateCredentialRequest,
  TestCredentialResponse,
  CredentialReference,
} from '@/types/credential';
import * as credentialService from '@/services/credentialService';

// ============================================================================
// Types
// ============================================================================

interface CredentialsState {
  // Data
  credentials: Credential[];
  integrations: IntegrationCategory[];
  credentialReferences: CredentialReference[];

  // Loading states
  isLoading: boolean;
  isCreating: boolean;
  isUpdating: boolean;
  isDeleting: boolean;
  isTesting: string | null; // Credential ID being tested

  // Error handling
  error: string | null;

  // UI state
  selectedIntegrationType: string | null;
  selectedCredentialId: string | null;

  // Actions
  fetchIntegrations: () => Promise<void>;
  fetchCredentials: () => Promise<void>;
  fetchCredentialReferences: (integrationTypes?: string[]) => Promise<void>;
  createCredential: (data: CreateCredentialRequest) => Promise<Credential | null>;
  updateCredential: (id: string, data: UpdateCredentialRequest) => Promise<Credential | null>;
  deleteCredential: (id: string) => Promise<boolean>;
  testCredential: (id: string) => Promise<TestCredentialResponse | null>;

  // UI actions
  setSelectedIntegrationType: (type: string | null) => void;
  setSelectedCredentialId: (id: string | null) => void;
  clearError: () => void;
  reset: () => void;
}

// ============================================================================
// Initial State
// ============================================================================

const initialState = {
  credentials: [],
  integrations: [],
  credentialReferences: [],
  isLoading: false,
  isCreating: false,
  isUpdating: false,
  isDeleting: false,
  isTesting: null,
  error: null,
  selectedIntegrationType: null,
  selectedCredentialId: null,
};

// ============================================================================
// Store
// ============================================================================

export const useCredentialsStore = create<CredentialsState>()(
  devtools(
    (set, get) => ({
      ...initialState,

      // ========================================================================
      // Data Fetching
      // ========================================================================

      fetchIntegrations: async () => {
        try {
          set({ isLoading: true, error: null });
          const integrations = await credentialService.getIntegrations();
          set({ integrations, isLoading: false });
        } catch (error) {
          console.error('Failed to fetch integrations:', error);
          set({
            error: error instanceof Error ? error.message : 'Failed to fetch integrations',
            isLoading: false,
          });
        }
      },

      fetchCredentials: async () => {
        try {
          set({ isLoading: true, error: null });
          const credentials = await credentialService.listCredentials();
          set({ credentials, isLoading: false });
        } catch (error) {
          console.error('Failed to fetch credentials:', error);
          set({
            error: error instanceof Error ? error.message : 'Failed to fetch credentials',
            isLoading: false,
          });
        }
      },

      fetchCredentialReferences: async (integrationTypes?: string[]) => {
        try {
          const credentialReferences =
            await credentialService.getCredentialReferences(integrationTypes);
          set({ credentialReferences });
        } catch (error) {
          console.error('Failed to fetch credential references:', error);
        }
      },

      // ========================================================================
      // CRUD Operations
      // ========================================================================

      createCredential: async (data: CreateCredentialRequest) => {
        try {
          set({ isCreating: true, error: null });
          const credential = await credentialService.createCredential(data);

          // Add to local state
          set(state => ({
            credentials: [...state.credentials, credential],
            isCreating: false,
          }));

          return credential;
        } catch (error) {
          console.error('Failed to create credential:', error);
          set({
            error: error instanceof Error ? error.message : 'Failed to create credential',
            isCreating: false,
          });
          return null;
        }
      },

      updateCredential: async (id: string, data: UpdateCredentialRequest) => {
        try {
          set({ isUpdating: true, error: null });
          const updated = await credentialService.updateCredential(id, data);

          // Update local state
          set(state => ({
            credentials: state.credentials.map(c => (c.id === id ? updated : c)),
            isUpdating: false,
          }));

          return updated;
        } catch (error) {
          console.error('Failed to update credential:', error);
          set({
            error: error instanceof Error ? error.message : 'Failed to update credential',
            isUpdating: false,
          });
          return null;
        }
      },

      deleteCredential: async (id: string) => {
        try {
          set({ isDeleting: true, error: null });
          await credentialService.deleteCredential(id);

          // Remove from local state
          set(state => ({
            credentials: state.credentials.filter(c => c.id !== id),
            isDeleting: false,
            selectedCredentialId:
              state.selectedCredentialId === id ? null : state.selectedCredentialId,
          }));

          return true;
        } catch (error) {
          console.error('Failed to delete credential:', error);
          set({
            error: error instanceof Error ? error.message : 'Failed to delete credential',
            isDeleting: false,
          });
          return false;
        }
      },

      testCredential: async (id: string) => {
        try {
          set({ isTesting: id, error: null });
          const result = await credentialService.testCredential(id);

          // Update local credential with test status
          set(state => ({
            credentials: state.credentials.map(c =>
              c.id === id
                ? {
                    ...c,
                    metadata: {
                      ...c.metadata,
                      testStatus: result.success ? 'success' : 'failed',
                      lastTestAt: new Date().toISOString(),
                    },
                  }
                : c
            ),
            isTesting: null,
          }));

          return result;
        } catch (error) {
          console.error('Failed to test credential:', error);
          set({
            error: error instanceof Error ? error.message : 'Failed to test credential',
            isTesting: null,
          });
          return null;
        }
      },

      // ========================================================================
      // UI Actions
      // ========================================================================

      setSelectedIntegrationType: (type: string | null) => {
        set({ selectedIntegrationType: type });
      },

      setSelectedCredentialId: (id: string | null) => {
        set({ selectedCredentialId: id });
      },

      clearError: () => {
        set({ error: null });
      },

      reset: () => {
        set(initialState);
      },
    }),
    { name: 'credentials-store' }
  )
);

// ============================================================================
// Selectors (for optimized re-renders)
// ============================================================================

export const useCredentials = () => useCredentialsStore(state => state.credentials);

export const useIntegrations = () => useCredentialsStore(state => state.integrations);

export const useCredentialsByType = (integrationType: string) =>
  useCredentialsStore(state =>
    state.credentials.filter(c => c.integrationType === integrationType)
  );

export const useCredentialById = (id: string | null) =>
  useCredentialsStore(state => (id ? state.credentials.find(c => c.id === id) : undefined));

export const useIntegrationById = (id: string | null) =>
  useCredentialsStore(state => {
    if (!id) return undefined;
    for (const category of state.integrations) {
      const integration = category.integrations.find(i => i.id === id);
      if (integration) return integration;
    }
    return undefined;
  });

export const useCredentialsLoading = () =>
  useCredentialsStore(state => ({
    isLoading: state.isLoading,
    isCreating: state.isCreating,
    isUpdating: state.isUpdating,
    isDeleting: state.isDeleting,
    isTesting: state.isTesting,
  }));

export const useCredentialsError = () => useCredentialsStore(state => state.error);

// ============================================================================
// Helper: Get configured integration types
// ============================================================================

export const useConfiguredIntegrationTypes = () =>
  useCredentialsStore(state => {
    const types = new Set<string>();
    for (const cred of state.credentials) {
      types.add(cred.integrationType);
    }
    return Array.from(types);
  });

// ============================================================================
// Helper: Get credentials count by integration type
// ============================================================================

export const useCredentialsCountByType = () =>
  useCredentialsStore(state => {
    const counts: Record<string, number> = {};
    for (const cred of state.credentials) {
      counts[cred.integrationType] = (counts[cred.integrationType] || 0) + 1;
    }
    return counts;
  });
