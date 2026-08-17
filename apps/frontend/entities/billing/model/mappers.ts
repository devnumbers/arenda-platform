import type { components } from '@/shared/api/dto';
import type { TariffName } from '@/shared/model/tariff';
import type {
  AddPaymentMethodResult,
  ChangeTariffResult,
  PaymentMethod,
  PaymentPeriod,
  PaymentStatus,
  Subscription,
  SubscriptionPayment,
  SubscriptionStatus,
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
    name: response.name as TariffName,
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
    status: response.status as SubscriptionStatus,
    tariff: mapTariffResponse(response.tariff),
    pendingTariff: response.pendingTariff
      ? mapTariffResponse(response.pendingTariff)
      : undefined,
    autoRenewEnabled: response.autoRenewEnabled,
    validUntil: response.validUntil ?? undefined,
    currentPeriod: response.currentPeriod
      ? (response.currentPeriod as PaymentPeriod)
      : undefined,
    activePaymentMethod: response.activePaymentMethod
      ? mapPaymentMethodResponse(response.activePaymentMethod)
      : undefined,
    pendingChangeAt: response.pendingChangeAt ?? undefined,
    pendingPeriod: response.pendingPeriod
      ? (response.pendingPeriod as PaymentPeriod)
      : undefined,
  };
}

export function mapSubscriptionPaymentResponse(
  response: SubscriptionPaymentResponse,
): SubscriptionPayment {
  return {
    id: response.id,
    tariff: mapTariffResponse(response.tariff),
    period: response.period as PaymentPeriod,
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
