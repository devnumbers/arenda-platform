import type { components } from '@/shared/api/generated';

type Subscription = components['schemas']['Subscription'];
type SubscriptionStatus = Subscription['status'];

export function isSubscriptionReadonly(subscription: Subscription | null | undefined): boolean {
  if (subscription === null || subscription === undefined) {
    return false;
  }

  const status: SubscriptionStatus = subscription.status;

  if (status === 'blocked' || status === 'cancelled') {
    return true;
  }

  if (status === 'grace') {
    const validUntil = subscription.validUntil;
    if (validUntil) {
      return new Date(validUntil) < new Date();
    }
  }

  return false;
}
