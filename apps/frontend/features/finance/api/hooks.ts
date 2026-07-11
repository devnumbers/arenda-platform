'use client';

import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { financeKeys } from './keys';
import type { components } from '@/shared/api/generated';

type FinanceReportResponse = components['schemas']['FinanceReportResponse'];

export function useFinanceReport(
  from?: string,
  to?: string,
): UseQueryResult<FinanceReportResponse, ApiError> {
  const url =
    from && to
      ? `/finance/report?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`
      : '/finance/report';
  return useQuery({
    queryKey: financeKeys.report(from, to),
    queryFn: () => apiClient<FinanceReportResponse>(url),
    enabled: true,
  });
}
