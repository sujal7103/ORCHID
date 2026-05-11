import { useState, useEffect } from 'react';
import { Modal } from '@/components/design-system/feedback/Modal/Modal';
import { Eye, EyeOff } from 'lucide-react';
import type { ProviderConfig, CreateProviderRequest } from '@/types/admin';

export interface ProviderFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: (data: CreateProviderRequest) => Promise<void>;
  provider?: ProviderConfig | null;
  mode?: 'create' | 'edit';
}

export const ProviderForm: React.FC<ProviderFormProps> = ({
  isOpen,
  onClose,
  onSave,
  provider = null,
  mode = 'create',
}) => {
  const [formData, setFormData] = useState<CreateProviderRequest>({
    name: '',
    base_url: '',
    api_key: '',
    enabled: true,
    audio_only: false,
    image_only: false,
    image_edit_only: false,
    secure: false,
    default_model: '',
    system_prompt: '',
    favicon: '',
  });

  const [showApiKey, setShowApiKey] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (isOpen) {
      if (provider) {
        setFormData({
          name: provider.name,
          base_url: provider.base_url,
          api_key: provider.api_key,
          enabled: provider.enabled,
          audio_only: provider.audio_only || false,
          image_only: provider.image_only || false,
          image_edit_only: provider.image_edit_only || false,
          secure: provider.secure || false,
          default_model: provider.default_model || '',
          system_prompt: provider.system_prompt || '',
          favicon: provider.favicon || '',
        });
      } else {
        // Reset form for create mode
        setFormData({
          name: '',
          base_url: '',
          api_key: '',
          enabled: true,
          audio_only: false,
          image_only: false,
          image_edit_only: false,
          secure: false,
          default_model: '',
          system_prompt: '',
          favicon: '',
        });
      }
      setErrors({});
      setShowApiKey(false);
    }
  }, [isOpen, provider]);

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.name.trim()) {
      newErrors.name = 'Provider name is required';
    } else if (formData.name.length > 255) {
      newErrors.name = 'Provider name must be less than 255 characters';
    }

    if (!formData.base_url.trim()) {
      newErrors.base_url = 'Base URL is required';
    } else {
      try {
        new URL(formData.base_url);
      } catch {
        newErrors.base_url = 'Must be a valid URL';
      }
    }

    if (!formData.api_key.trim()) {
      newErrors.api_key = 'API key is required';
    }

    // Validate that only one special type is selected
    const specialTypes = [formData.audio_only, formData.image_only, formData.image_edit_only];
    const selectedCount = specialTypes.filter(Boolean).length;
    if (selectedCount > 1) {
      newErrors.special_type = 'Only one special type can be selected';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    setIsSubmitting(true);
    try {
      await onSave(formData);
      onClose();
    } catch (error) {
      console.error('Failed to save provider:', error);
      setErrors({ submit: 'Failed to save provider. Please try again.' });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSpecialTypeChange = (
    type: 'audio_only' | 'image_only' | 'image_edit_only',
    checked: boolean
  ) => {
    setFormData(prev => ({
      ...prev,
      audio_only: type === 'audio_only' ? checked : false,
      image_only: type === 'image_only' ? checked : false,
      image_edit_only: type === 'image_edit_only' ? checked : false,
    }));
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={mode === 'create' ? 'Add New Provider' : `Edit Provider: ${provider?.name}`}
      size="lg"
      closeOnBackdrop={false}
      closeOnEscape={true}
    >
      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Basic Information */}
        <div className="space-y-4">
          <h4 className="text-sm font-semibold text-[var(--color-text-primary)] uppercase tracking-wide">
            Basic Information
          </h4>

          <div>
            <label
              htmlFor="name"
              className="block text-sm font-medium text-[var(--color-text-primary)] mb-1"
            >
              Provider Name *
            </label>
            <input
              type="text"
              id="name"
              value={formData.name}
              onChange={e => setFormData({ ...formData, name: e.target.value })}
              className={`w-full px-3 py-2 bg-[var(--color-surface)] rounded-lg text-[var(--color-text-primary)] placeholder-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] ${
                errors.name ? 'border border-[var(--color-error)]' : ''
              }`}
              placeholder="e.g., OpenAI, Anthropic, Custom Provider"
              disabled={isSubmitting}
            />
            {errors.name && <p className="text-xs text-[var(--color-error)] mt-1">{errors.name}</p>}
          </div>

          <div>
            <label
              htmlFor="base_url"
              className="block text-sm font-medium text-[var(--color-text-primary)] mb-1"
            >
              API Base URL *
            </label>
            <input
              type="text"
              id="base_url"
              value={formData.base_url}
              onChange={e => setFormData({ ...formData, base_url: e.target.value })}
              className={`w-full px-3 py-2 bg-[var(--color-surface)] rounded-lg text-[var(--color-text-primary)] placeholder-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] ${
                errors.base_url ? 'border border-[var(--color-error)]' : ''
              }`}
              placeholder="https://api.provider.com/v1"
              disabled={isSubmitting}
            />
            {errors.base_url && (
              <p className="text-xs text-[var(--color-error)] mt-1">{errors.base_url}</p>
            )}
          </div>

          <div>
            <label
              htmlFor="api_key"
              className="block text-sm font-medium text-[var(--color-text-primary)] mb-1"
            >
              API Key *
            </label>
            <div className="relative">
              <input
                type={showApiKey ? 'text' : 'password'}
                id="api_key"
                value={formData.api_key}
                onChange={e => setFormData({ ...formData, api_key: e.target.value })}
                className={`w-full px-3 py-2 pr-10 bg-[var(--color-surface)] rounded-lg text-[var(--color-text-primary)] placeholder-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] ${
                  errors.api_key ? 'border border-[var(--color-error)]' : ''
                }`}
                placeholder="sk-..."
                disabled={isSubmitting}
              />
              <button
                type="button"
                onClick={() => setShowApiKey(!showApiKey)}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 text-[var(--color-text-tertiary)] hover:text-[var(--color-text-primary)] transition-colors"
                tabIndex={-1}
              >
                {showApiKey ? <EyeOff size={18} /> : <Eye size={18} />}
              </button>
            </div>
            {errors.api_key && (
              <p className="text-xs text-[var(--color-error)] mt-1">{errors.api_key}</p>
            )}
          </div>
        </div>

        {/* Special Provider Types */}
        <div className="space-y-3">
          <h4 className="text-sm font-semibold text-[var(--color-text-primary)] uppercase tracking-wide">
            Special Provider Type
          </h4>
          <p className="text-xs text-[var(--color-text-tertiary)]">
            Select if this provider handles a specific type of content (only one can be selected)
          </p>

          <div className="space-y-2">
            <label className="flex items-center gap-3 p-3 bg-[var(--color-surface)] rounded-lg hover:bg-[var(--color-surface-hover)] transition-colors cursor-pointer">
              <input
                type="checkbox"
                checked={formData.audio_only || false}
                onChange={e => handleSpecialTypeChange('audio_only', e.target.checked)}
                className="w-4 h-4 accent-[var(--color-accent)]"
                disabled={isSubmitting}
              />
              <div>
                <div className="text-sm font-medium text-[var(--color-text-primary)]">
                  Audio Only (Transcription)
                </div>
                <div className="text-xs text-[var(--color-text-tertiary)]">
                  Provider handles audio transcription and speech-to-text
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3 bg-[var(--color-surface)] rounded-lg hover:bg-[var(--color-surface-hover)] transition-colors cursor-pointer">
              <input
                type="checkbox"
                checked={formData.image_only || false}
                onChange={e => handleSpecialTypeChange('image_only', e.target.checked)}
                className="w-4 h-4 accent-[var(--color-accent)]"
                disabled={isSubmitting}
              />
              <div>
                <div className="text-sm font-medium text-[var(--color-text-primary)]">
                  Image Generation
                </div>
                <div className="text-xs text-[var(--color-text-tertiary)]">
                  Provider generates images from text descriptions
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3 bg-[var(--color-surface)] rounded-lg hover:bg-[var(--color-surface-hover)] transition-colors cursor-pointer">
              <input
                type="checkbox"
                checked={formData.image_edit_only || false}
                onChange={e => handleSpecialTypeChange('image_edit_only', e.target.checked)}
                className="w-4 h-4 accent-[var(--color-accent)]"
                disabled={isSubmitting}
              />
              <div>
                <div className="text-sm font-medium text-[var(--color-text-primary)]">
                  Image Editing
                </div>
                <div className="text-xs text-[var(--color-text-tertiary)]">
                  Provider edits or modifies existing images
                </div>
              </div>
            </label>
          </div>
          {errors.special_type && (
            <p className="text-xs text-[var(--color-error)] mt-1">{errors.special_type}</p>
          )}
        </div>

        {/* Security & Settings */}
        <div className="space-y-3">
          <h4 className="text-sm font-semibold text-[var(--color-text-primary)] uppercase tracking-wide">
            Security & Settings
          </h4>

          <label className="flex items-center gap-3 p-3 bg-[var(--color-surface)] rounded-lg hover:bg-[var(--color-surface-hover)] transition-colors cursor-pointer">
            <input
              type="checkbox"
              checked={formData.secure || false}
              onChange={e => setFormData({ ...formData, secure: e.target.checked })}
              className="w-4 h-4 accent-[var(--color-accent)]"
              disabled={isSubmitting}
            />
            <div>
              <div className="text-sm font-medium text-[var(--color-text-primary)]">
                Private/Secure Provider
              </div>
              <div className="text-xs text-[var(--color-text-tertiary)]">
                Provider runs in TEE or doesn't store data (enhanced privacy)
              </div>
            </div>
          </label>

          <label className="flex items-center gap-3 p-3 bg-[var(--color-surface)] rounded-lg hover:bg-[var(--color-surface-hover)] transition-colors cursor-pointer">
            <input
              type="checkbox"
              checked={formData.enabled}
              onChange={e => setFormData({ ...formData, enabled: e.target.checked })}
              className="w-4 h-4 accent-[var(--color-accent)]"
              disabled={isSubmitting}
            />
            <div>
              <div className="text-sm font-medium text-[var(--color-text-primary)]">Enabled</div>
              <div className="text-xs text-[var(--color-text-tertiary)]">
                Provider is active and available for use
              </div>
            </div>
          </label>
        </div>

        {/* Optional Metadata */}
        <div className="space-y-4">
          <h4 className="text-sm font-semibold text-[var(--color-text-primary)] uppercase tracking-wide">
            Optional Metadata
          </h4>

          <div>
            <label
              htmlFor="default_model"
              className="block text-sm font-medium text-[var(--color-text-primary)] mb-1"
            >
              Default Model
            </label>
            <input
              type="text"
              id="default_model"
              value={formData.default_model || ''}
              onChange={e => setFormData({ ...formData, default_model: e.target.value })}
              className="w-full px-3 py-2 bg-[var(--color-surface)] rounded-lg text-[var(--color-text-primary)] placeholder-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
              placeholder="gpt-4, claude-3-opus, etc."
              disabled={isSubmitting}
            />
          </div>

          <div>
            <label
              htmlFor="favicon"
              className="block text-sm font-medium text-[var(--color-text-primary)] mb-1"
            >
              Favicon URL
            </label>
            <input
              type="text"
              id="favicon"
              value={formData.favicon || ''}
              onChange={e => setFormData({ ...formData, favicon: e.target.value })}
              className="w-full px-3 py-2 bg-[var(--color-surface)] rounded-lg text-[var(--color-text-primary)] placeholder-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
              placeholder="https://example.com/icon.png"
              disabled={isSubmitting}
            />
          </div>

          <div>
            <label
              htmlFor="system_prompt"
              className="block text-sm font-medium text-[var(--color-text-primary)] mb-1"
            >
              System Prompt
            </label>
            <textarea
              id="system_prompt"
              value={formData.system_prompt || ''}
              onChange={e => setFormData({ ...formData, system_prompt: e.target.value })}
              className="w-full px-3 py-2 bg-[var(--color-surface)] rounded-lg text-[var(--color-text-primary)] placeholder-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] resize-none"
              placeholder="Optional system prompt to prepend to all requests"
              rows={3}
              disabled={isSubmitting}
            />
          </div>
        </div>

        {/* Form Actions */}
        {errors.submit && (
          <div className="bg-[var(--color-error-light)] text-[var(--color-error)] px-4 py-3 rounded-lg text-sm">
            {errors.submit}
          </div>
        )}

        <div className="flex justify-end gap-3 pt-4 border-t border-[var(--color-surface-hover)]">
          <button
            type="button"
            onClick={onClose}
            disabled={isSubmitting}
            className="px-4 py-2 text-sm font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-hover)] rounded-lg transition-colors disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={isSubmitting}
            className="px-4 py-2 text-sm font-medium text-white bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isSubmitting ? 'Saving...' : mode === 'create' ? 'Create Provider' : 'Save Changes'}
          </button>
        </div>
      </form>
    </Modal>
  );
};
