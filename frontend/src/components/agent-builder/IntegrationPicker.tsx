import { useState, useEffect } from 'react';
import { Search, KeyRound, ExternalLink, CheckCircle, AlertCircle, Loader2 } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useCredentialsStore } from '@/store/useCredentialsStore';
import { IntegrationIcon } from '@/components/credentials/IntegrationIcon';
import type { CredentialReference, Integration } from '@/types/credential';

interface IntegrationPickerProps {
  /**
   * Currently selected credential IDs
   */
  selectedCredentials: string[];

  /**
   * Callback when credential selection changes
   */
  onSelectionChange: (credentialIds: string[]) => void;

  /**
   * Optional: Filter to specific tool names
   * If provided, only show integrations that support these tools
   */
  toolFilter?: string[];

  /**
   * Optional: Compact mode for smaller spaces
   */
  compact?: boolean;
}

export function IntegrationPicker({
  selectedCredentials,
  onSelectionChange,
  toolFilter,
  compact = false,
}: IntegrationPickerProps) {
  const {
    credentialReferences,
    integrations,
    isLoading,
    fetchCredentialReferences,
    fetchIntegrations,
  } = useCredentialsStore();

  const [searchQuery, setSearchQuery] = useState('');

  // Fetch data on mount
  useEffect(() => {
    fetchIntegrations();
    fetchCredentialReferences();
  }, [fetchIntegrations, fetchCredentialReferences]);

  // Group credentials by integration type
  const groupedCredentials = credentialReferences.reduce(
    (acc, cred) => {
      if (!acc[cred.integrationType]) {
        acc[cred.integrationType] = [];
      }
      acc[cred.integrationType].push(cred);
      return acc;
    },
    {} as Record<string, CredentialReference[]>
  );

  // Get integration details
  const getIntegration = (integrationType: string): Integration | undefined => {
    for (const category of integrations) {
      const integration = category.integrations.find(i => i.id === integrationType);
      if (integration) return integration;
    }
    return undefined;
  };

  // Filter integrations based on tool filter and search
  const filteredIntegrationTypes = Object.keys(groupedCredentials).filter(type => {
    const integration = getIntegration(type);
    if (!integration) return false;

    // Filter by tools if specified
    if (toolFilter && toolFilter.length > 0) {
      const hasMatchingTool = integration.tools.some(tool => toolFilter.includes(tool));
      if (!hasMatchingTool) return false;
    }

    // Filter by search
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      return integration.name.toLowerCase().includes(query) || type.toLowerCase().includes(query);
    }

    return true;
  });

  // Auto-select single credentials for each integration type
  // This provides a better UX - if user only has one Discord webhook, select it automatically
  useEffect(() => {
    // Only auto-select if nothing is selected yet and not loading
    if (selectedCredentials.length > 0 || isLoading) return;

    // Only auto-select when we have a tool filter (i.e., tools are selected for this block)
    if (!toolFilter || toolFilter.length === 0) return;

    // Collect credentials to auto-select
    const credentialsToSelect: string[] = [];

    for (const integrationType of filteredIntegrationTypes) {
      const credentials = groupedCredentials[integrationType];
      // If this integration has exactly one credential, auto-select it
      if (credentials && credentials.length === 1) {
        credentialsToSelect.push(credentials[0].id);
      }
    }

    // Only call onSelectionChange if we have credentials to select
    if (credentialsToSelect.length > 0) {
      onSelectionChange(credentialsToSelect);
    }
  }, [
    filteredIntegrationTypes,
    groupedCredentials,
    selectedCredentials.length,
    isLoading,
    toolFilter,
    onSelectionChange,
  ]);

  // Toggle credential selection
  const toggleCredential = (credentialId: string) => {
    const newSelection = selectedCredentials.includes(credentialId)
      ? selectedCredentials.filter(id => id !== credentialId)
      : [...selectedCredentials, credentialId];
    onSelectionChange(newSelection);
  };

  // Select all credentials for an integration type
  const selectAllForType = (integrationType: string) => {
    const typeCredentials = groupedCredentials[integrationType] || [];
    const typeIds = typeCredentials.map(c => c.id);
    const allSelected = typeIds.every(id => selectedCredentials.includes(id));

    if (allSelected) {
      // Deselect all
      onSelectionChange(selectedCredentials.filter(id => !typeIds.includes(id)));
    } else {
      // Select all
      const newSelection = [...new Set([...selectedCredentials, ...typeIds])];
      onSelectionChange(newSelection);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-4">
        <Loader2 size={20} className="animate-spin text-[var(--color-accent)]" />
      </div>
    );
  }

  if (credentialReferences.length === 0) {
    return (
      <div className="p-4 rounded-lg bg-[var(--color-bg-primary)] border border-[var(--color-border)]">
        <div className="flex items-center gap-3 text-[var(--color-text-secondary)]">
          <KeyRound size={20} />
          <div>
            <p className="text-sm font-medium">No credentials configured</p>
            <p className="text-xs mt-0.5">
              <Link
                to="/credentials"
                className="text-[var(--color-accent)] hover:underline inline-flex items-center gap-1"
              >
                Add credentials <ExternalLink size={12} />
              </Link>{' '}
              to enable external integrations
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <KeyRound size={16} className="text-[var(--color-accent)]" />
          <span className="text-sm font-medium text-[var(--color-text-primary)]">Credentials</span>
          <span className="text-xs text-[var(--color-text-tertiary)]">
            ({selectedCredentials.length} selected)
          </span>
        </div>
        <Link
          to="/credentials"
          className="text-xs text-[var(--color-accent)] hover:underline flex items-center gap-1"
        >
          Manage <ExternalLink size={10} />
        </Link>
      </div>

      {/* Search */}
      {!compact && Object.keys(groupedCredentials).length > 3 && (
        <div className="relative">
          <Search
            size={14}
            className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-text-tertiary)]"
          />
          <input
            type="text"
            placeholder="Search integrations..."
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="w-full pl-8 pr-3 py-1.5 rounded-md bg-[var(--color-bg-primary)] border border-[var(--color-border)] text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-tertiary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
          />
        </div>
      )}

      {/* Credentials List */}
      <div className="space-y-2 max-h-64 overflow-y-auto">
        {filteredIntegrationTypes.length === 0 ? (
          <p className="text-sm text-[var(--color-text-tertiary)] text-center py-2">
            {searchQuery
              ? 'No matching credentials found'
              : 'No credentials available for selected tools'}
          </p>
        ) : (
          filteredIntegrationTypes.map(integrationType => {
            const integration = getIntegration(integrationType);
            const credentials = groupedCredentials[integrationType];
            const allSelected = credentials.every(c => selectedCredentials.includes(c.id));
            const someSelected = credentials.some(c => selectedCredentials.includes(c.id));

            return (
              <div
                key={integrationType}
                className="rounded-lg bg-[var(--color-bg-primary)] border border-[var(--color-border)] overflow-hidden"
              >
                {/* Integration Header */}
                <button
                  onClick={() => selectAllForType(integrationType)}
                  className="w-full flex items-center gap-2 px-3 py-2 hover:bg-[var(--color-surface-hover)] transition-colors"
                >
                  <div
                    className={`w-4 h-4 rounded border-2 flex items-center justify-center transition-colors ${
                      allSelected
                        ? 'bg-[var(--color-accent)] border-[var(--color-accent)]'
                        : someSelected
                          ? 'border-[var(--color-accent)] bg-[var(--color-accent)]/20'
                          : 'border-[var(--color-border)]'
                    }`}
                  >
                    {(allSelected || someSelected) && (
                      <CheckCircle
                        size={10}
                        className={allSelected ? 'text-white' : 'text-[var(--color-accent)]'}
                      />
                    )}
                  </div>
                  <IntegrationIcon integrationId={integrationType} size={18} forceColor="white" />
                  <span className="text-sm font-medium text-[var(--color-text-primary)] flex-1 text-left">
                    {integration?.name || integrationType}
                  </span>
                  <span className="text-xs text-[var(--color-text-tertiary)]">
                    {credentials.length}
                  </span>
                </button>

                {/* Individual Credentials */}
                {credentials.length > 1 && (
                  <div className="border-t border-[var(--color-border)] px-3 py-1.5 space-y-1">
                    {credentials.map(cred => {
                      const isSelected = selectedCredentials.includes(cred.id);
                      return (
                        <button
                          key={cred.id}
                          onClick={() => toggleCredential(cred.id)}
                          className="w-full flex items-center gap-2 px-2 py-1 rounded hover:bg-[var(--color-surface-hover)] transition-colors"
                        >
                          <div
                            className={`w-3.5 h-3.5 rounded border-2 flex items-center justify-center transition-colors ${
                              isSelected
                                ? 'bg-[var(--color-accent)] border-[var(--color-accent)]'
                                : 'border-[var(--color-border)]'
                            }`}
                          >
                            {isSelected && <CheckCircle size={8} className="text-white" />}
                          </div>
                          <span className="text-xs text-[var(--color-text-secondary)] flex-1 text-left truncate">
                            {cred.name}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>

      {/* Help text */}
      {!compact && (
        <p className="text-xs text-[var(--color-text-tertiary)]">
          Selected credentials will be available to tools during agent execution
        </p>
      )}
    </div>
  );
}

export default IntegrationPicker;
