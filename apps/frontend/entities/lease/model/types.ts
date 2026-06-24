export type LeaseStatus = 'active' | 'completed' | 'cancelled';

export type Lease = {
  readonly id: string;
  readonly propertyId: string;
  readonly tenantName: string;
  readonly rentKopecks: number;
  readonly startDate: string;
  readonly endDate?: string;
  readonly status: LeaseStatus;
};
