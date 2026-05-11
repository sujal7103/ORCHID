/**
 * @deprecated This component is deprecated and will not be shown to users.
 * The onboarding guidance popup after workflow generation is no longer needed.
 * This component is kept for backwards compatibility but returns null.
 */

import { useEffect } from 'react';
import { useAgentBuilderStore } from '@/store/useAgentBuilderStore';

interface OnboardingGuidanceProps {
  onRun: () => void;
  onDeploy: () => void;
}

export function OnboardingGuidance({ onRun, onDeploy }: OnboardingGuidanceProps) {
  const { setShowOnboardingGuidance } = useAgentBuilderStore();

  // Auto-hide immediately
  useEffect(() => {
    setShowOnboardingGuidance(false);
  }, [setShowOnboardingGuidance]);

  // Return null - this component is deprecated and no longer shown
  return null;
}
