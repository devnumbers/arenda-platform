export type LeaseStatus =
  | 'awaiting_start'
  | 'active'
  | 'requires_action'
  | 'completed'
  | 'archived';

export type Lease = {
  readonly id: string;
  readonly propertyId: string;
  readonly tenantContactId?: string;
  readonly tenantName: string;
  readonly rentKopecks: number;
  readonly startDate: string;
  readonly endDate?: string;
  readonly status: LeaseStatus;
};
