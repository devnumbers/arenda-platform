export const financeKeys = {
  report: (from: string, to: string) => ['finance', 'report', from, to] as const,
};
