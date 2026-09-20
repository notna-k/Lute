/**
 * Tone → class map for option tags. Kept out of the component files so React
 * Fast Refresh keeps working on them.
 */
export const TONE_TAG: Record<string, string> = {
  neutral: 'text-fg-muted bg-surface-hover',
  success: 'text-success-fg bg-success-subtle',
  warning: 'text-warning-fg bg-warning-subtle',
  danger: 'text-danger-fg bg-danger-subtle',
};

export const TONES = ['neutral', 'success', 'warning', 'danger'] as const;
