export const ROUTES = {
  login: '/login',
  properties: '/properties',
  property: (id: string) => `/properties/${id}`,
  propertyArchive: '/properties/archive',
  propertyNew: '/properties/new',
  propertyEdit: (id: string) => `/properties/${id}/edit`,
  propertyContactsNew: (id: string) => `/properties/${id}/contacts/new`,
  propertyContactEdit: (propertyId: string, contactId: string) =>
    `/properties/${propertyId}/contacts/${contactId}/edit`,
  /** Экран «Платежи объекта» — новый хром (#463). */
  propertyPayments: (id: string) => `/properties/${id}/payments`,
  /** Страница платежа (#465): карточка, мутации, секции вхождений. */
  propertyPayment: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}`,
  /** Подэкраны страницы платежа (#466): график, история, просрочки. */
  propertyPaymentSchedule: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}/schedule`,
  propertyPaymentHistory: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}/history`,
  propertyPaymentOverdue: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}/overdue`,
  /** Экран правки платежа (#467): форма поверх правила, удаление — там же. */
  propertyPaymentEdit: (id: string, paymentId: string) =>
    `/properties/${id}/payments/${paymentId}/edit`,
  /** Визард создания платежа; тип выбирается в шите выбора «Платёж / Автоплатёж». */
  propertyPaymentNew: (id: string, type: 'payment' | 'autopayment') =>
    `/properties/${id}/payments/new?type=${type}`,
  profile: '/profile',
  profilePersonal: '/profile/personal',
  profileNotifications: '/profile/notifications',
  profileAccount: '/profile/account',
  profileChangePhone: '/profile/account/phone',
  profileTariff: '/profile/tariff',
  profileTariffChange: '/profile/tariff/change',
  profileTariffChangeSuccess: '/profile/tariff/change/success',
  profilePaymentMethods: '/profile/tariff/payment-methods',
  profilePaymentMethodsAdd: '/profile/tariff/payment-methods/add',
  profilePayments: '/profile/tariff/payments',
  profilePaymentDetail: (id: string) => `/profile/tariff/payments/${id}`,
  profileInfo: '/profile/info',
  profilePrivacy: '/profile/info/privacy',
  profileTerms: '/profile/info/terms',
} as const;
