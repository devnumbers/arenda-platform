export type LeaseStatus =
  | 'awaiting_start'
  | 'active'
  | 'requires_action'
  | 'completed'
  | 'archived';

export type TenantContact = {
  readonly id: string;
  readonly name: string;
  readonly surname?: string | null;
  readonly patronymic?: string | null;
  readonly phone?: string | null;
  readonly email?: string | null;
};

export type Lease = {
  readonly id: string;
  readonly propertyId: string | null;
  readonly tenantContactId?: string;
  readonly tenantName: string;
  readonly tenantContact?: TenantContact | null;
  readonly rentKopecks: number;
  readonly startDate: string;
  readonly endDate?: string;
  readonly paymentDay: number;
  readonly currentPeriodOverdue: boolean;
  readonly hasOverdue: boolean;
  readonly overdueSince?: string;
  readonly nextPaymentDate?: string;
  readonly status: LeaseStatus;
};
