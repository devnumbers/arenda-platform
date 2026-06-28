export const operationKeys = {
  byProperty: (
    propertyId: string,
    filters?: {
      type?: string | string[];
      status?: string | string[];
      category?: string | string[];
      from?: string;
      to?: string;
      recurring_operation_id?: string;
      limit?: string;
      offset?: string;
    },
  ) => ['properties', propertyId, 'operations', filters ?? {}] as const,
  detail: (id: string) => ['operations', id] as const,
  operations: (filters: {
    type?: string | string[];
    status?: string | string[];
    category?: string | string[];
    property_id?: string;
    from?: string;
    to?: string;
    recurring_operation_id?: string;
    lease_id?: string;
    limit?: string;
    offset?: string;
  }) => ['operations', filters] as const,
  reminders: (propertyId: string, operationId: string) =>
    [
      'properties',
      propertyId,
      'operations',
      operationId,
      'reminders',
    ] as const,
  summary: (propertyId: string) =>
    ['properties', propertyId, 'operations', 'summary'] as const,
  recurringByProperty: (propertyId: string) =>
    ['properties', propertyId, 'recurring-operations'] as const,
  recurringReminders: (propertyId: string, recurringOperationId: string) =>
    [
      'properties',
      propertyId,
      'recurring-operations',
      recurringOperationId,
      'reminders',
    ] as const,
};
