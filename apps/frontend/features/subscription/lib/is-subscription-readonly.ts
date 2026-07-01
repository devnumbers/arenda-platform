import type { components } from '@/shared/api/generated';

type Subscription = components['schemas']['Subscription'];
type SubscriptionStatus = Subscription['status'];

function isValidUntilExpired(subscription: Subscription): boolean {
  const validUntil = subscription.validUntil;
  if (!validUntil) {
    return false;
  }
  return new Date(validUntil) < new Date();
}

export function isSubscriptionReadonly(subscription: Subscription | null | undefined): boolean {
  if (subscription === null || subscription === undefined) {
    return false;
  }

  const status: SubscriptionStatus = subscription.status;

  if (status === 'blocked') {
    return true;
  }

  if (status === 'grace') {
    const validUntil = subscription.validUntil;
    if (!validUntil) {
      return true;
    }
    return new Date(validUntil) < new Date();
  }

  if (status === 'active' || status === 'cancelled') {
    return isValidUntilExpired(subscription);
  }

  return false;
}
