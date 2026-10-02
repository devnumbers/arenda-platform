import type { PaymentDraftType } from './use-payment-wizard-draft';

/**
 * Состояние контента шита «Добавить платёж» (спека #453, история 13):
 * выбор типа или модалка черновика — единый вход в создание платежа для
 * всех поверхностей (#1066). Фиксированный тип (кнопки «Добавить платеж» /
 * «Добавить автоплатеж» каталога, CTA «Добавить платёж» пустых «Объектов»)
 * минует фазу загрузки: шит с фиксированным типом открывают только при
 * существующем черновике этого типа — решает потребитель по hasDraft,
 * черновик читается синхронно на клиенте (useSyncExternalStore).
 */
export type PaymentAddSheetState =
  | { readonly kind: 'loading' }
  | { readonly kind: 'draft'; readonly draftType: PaymentDraftType }
  | { readonly kind: 'choice' };

export function paymentAddSheetState(
  loaded: boolean,
  latest: PaymentDraftType | undefined,
  fixedType?: PaymentDraftType,
): PaymentAddSheetState {
  if (fixedType !== undefined) {
    return { kind: 'draft', draftType: fixedType };
  }
  if (!loaded) {
    return { kind: 'loading' };
  }
  return latest !== undefined ? { kind: 'draft', draftType: latest } : { kind: 'choice' };
}
