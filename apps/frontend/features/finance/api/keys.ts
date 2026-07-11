export const financeKeys = {
  reports: () => ['finance', 'report'] as const,
  report: (from?: string, to?: string) =>
    [...financeKeys.reports(), from ?? 'all', to ?? 'all'] as const,
};
