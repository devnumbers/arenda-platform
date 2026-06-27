'use client';

import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { financeKeys } from './keys';
import type { components } from '@/shared/api/generated';

type FinanceReportResponse = components['schemas']['FinanceReportResponse'];

export function useFinanceReport(from: string, to: string) {
  return useQuery({
    queryKey: financeKeys.report(from, to),
    queryFn: () =>
      apiClient<FinanceReportResponse>(`/finance/report?from=${from}&to=${to}`),
    enabled: Boolean(from) && Boolean(to),
  });
}
