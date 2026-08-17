import type { Lease } from '@/shared/model/lease';

export type TenantContact = {
  readonly id: string;
  readonly ownerId: string;
  readonly name: string;
  readonly surname: string | null;
  readonly patronymic: string | null;
  readonly phone: string | null;
  readonly email: string | null;
  readonly comment: string | null;
  readonly isActive: boolean;
  readonly activeLease?: Lease;
  readonly lastLease?: Lease;
  readonly createdAt: string;
  readonly updatedAt: string;
};
