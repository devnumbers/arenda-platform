import type { components } from '@/shared/api/dto';
import type {
  AddPaymentMethodResult,
  ChangeTariffResult,
  PaymentMethod,
  PaymentStatus,
  Subscription,
  SubscriptionPayment,
  Tariff,
} from './types';

type TariffResponse = components['schemas']['Tariff'];
type PaymentMethodResponse = components['schemas']['PaymentMethod'];
type SubscriptionResponse = components['schemas']['Subscription'];
type SubscriptionPaymentResponse = components['schemas']['SubscriptionPayment'];
type ChangeTariffResponse = components['schemas']['ChangeTariffResponse'];
type AddPaymentMethodResponse = components['schemas']['AddPaymentMethodResponse'];

export function mapTariffResponse(response: TariffResponse): Tariff {
  return {
    id: response.name,
    name: response.name,
    monthlyPriceKopecks: response.monthlyPriceKopecks,
    yearlyPriceKopecks: response.yearlyPriceKopecks,
    activePropertyLimit: response.activePropertyLimit,
  };
}

export function mapPaymentMethodResponse(
  response: PaymentMethodResponse,
): PaymentMethod {
  return {
    id: response.id,
    displayMask: response.displayMask,
    provider: response.provider,
    isActive: response.isActive,
    createdAt: response.createdAt,
  };
}

export function mapSubscriptionResponse(
  response: SubscriptionResponse,
): Subscription {
  return {
    id: `${response.status}-${response.tariff.name}-${response.validUntil ?? ''}`,
    status: response.status,
    tariff: mapTariffResponse(response.tariff),
    pendingTariff: response.pendingTariff
      ? mapTariffResponse(response.pendingTariff)
      : undefined,
    autoRenewEnabled: response.autoRenewEnabled,
    validUntil: response.validUntil ?? undefined,
    currentPeriod: response.currentPeriod ?? undefined,
    activePaymentMethod: response.activePaymentMethod
      ? mapPaymentMethodResponse(response.activePaymentMethod)
      : undefined,
    pendingChangeAt: response.pendingChangeAt ?? undefined,
    pendingPeriod: response.pendingPeriod ?? undefined,
  };
}

export function mapSubscriptionPaymentResponse(
  response: SubscriptionPaymentResponse,
): SubscriptionPayment {
  return {
    id: response.id,
    tariff: mapTariffResponse(response.tariff),
    period: response.period,
    amountKopecks: response.amountKopecks,
    status: response.status as PaymentStatus,
    provider: response.provider,
    paymentUrl: response.paymentUrl ?? null,
    createdAt: response.createdAt,
  };
}

export function mapChangeTariffResponse(
  response: ChangeTariffResponse,
): ChangeTariffResult {
  return {
    paymentId: response.paymentId ?? undefined,
    confirmUrl: response.confirmUrl ?? undefined,
  };
}

export function mapAddPaymentMethodResponse(
  response: AddPaymentMethodResponse,
): AddPaymentMethodResult {
  return {
    confirmUrl: response.confirmUrl ?? undefined,
    paymentMethod: response.paymentMethod
      ? mapPaymentMethodResponse(response.paymentMethod)
      : undefined,
  };
}
