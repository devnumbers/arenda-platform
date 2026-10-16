import { type TranslationMessages } from 'ra-core';
import polyglotI18nProvider from 'ra-i18n-polyglot';
import russianMessages from 'ra-language-russian';

const customMessages = {
  resources: {
    users: {
      name: 'Пользователь |||| Пользователи',
      fields: {
        id: 'ID',
        phone: 'Телефон',
        email: 'Email',
        role: 'Роль',
        subscriptionStatus: 'Статус подписки',
        subscription_status: 'Статус подписки',
        createdAt: 'Создан',
        updatedAt: 'Обновлён',
        'subscription.status': 'Статус подписки',
        'subscription.tariff.name': 'Тариф',
        'subscription.validUntil': 'Действует до',
        'subscription.autoRenewEnabled': 'Автопродление',
        'stats.activePropertiesCount': 'Активных объектов',
        'stats.archivedPropertiesCount': 'Архивных объектов',
      },
    },
    subscriptionPayments: {
      name: 'Платёж |||| Платежи',
      fields: {
        id: 'ID',
        user_id: 'Пользователь',
        userId: 'Пользователь',
        userPhone: 'Телефон пользователя',
        user_phone: 'Телефон пользователя',
        status: 'Статус',
        amountKopecks: 'Сумма, ₽',
        refundedAmountKopecks: 'Возвращено, ₽',
        createdAt: 'Создан',
        updatedAt: 'Обновлён',
        succeededAt: 'Успешно оплачен',
        paymentMethodId: 'Способ оплаты',
        'tariff.name': 'Тариф',
        period: 'Период',
      },
    },
    tariffs: {
      name: 'Тариф |||| Тарифы',
      fields: {
        id: 'ID',
        name: 'Название',
        isActive: 'Активен',
        activePropertyLimit: 'Лимит активных объектов',
        monthlyPriceKopecks: 'Цена за месяц, ₽',
        yearlyPriceKopecks: 'Цена за год, ₽',
      },
    },
    properties: {
      name: 'Объект |||| Объекты',
      fields: {
        id: 'ID',
        ownerId: 'Владелец',
        name: 'Название',
        type: 'Тип',
        status: 'Статус',
        address: 'Адрес',
        description: 'Описание',
        q: 'Поиск',
        createdAt: 'Создан',
        updatedAt: 'Обновлён',
      },
    },
    auditLogs: {
      name: 'Запись журнала |||| Журнал действий',
      fields: {
        id: 'ID',
        actorId: 'Пользователь',
        actorRole: 'Роль',
        action: 'Действие',
        entityType: 'Сущность',
        entityId: 'ID сущности',
        context: 'Контекст',
        requestId: 'ID запроса',
        ip: 'IP',
        createdAt: 'Дата и время',
        date_from: 'С даты',
        date_to: 'По дату',
      },
    },
  },
};

const messages = { ...russianMessages } as Record<string, unknown>;
messages.resources = {
  ...(((russianMessages as Record<string, unknown>).resources ?? {}) as Record<string, unknown>),
  ...customMessages.resources,
};

export const i18nProvider = polyglotI18nProvider(() => messages as TranslationMessages, 'ru');
