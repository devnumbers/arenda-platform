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
        subscription_status: 'Статус подписки',
        createdAt: 'Создан',
        updatedAt: 'Обновлён',
        'subscription.status': 'Статус подписки',
        'subscription.tariff.name': 'Тариф',
        'subscription.validUntil': 'Действует до',
        'subscription.autoRenewEnabled': 'Автопродление',
        'stats.activePropertiesCount': 'Активных объектов',
        'stats.archivedPropertiesCount': 'Архивных объектов',
        'stats.leasesCount': 'Договоров',
        'stats.tenantContactsCount': 'Контактов',
        'stats.operationsCount': 'Операций',
      },
    },
    subscriptionPayments: {
      name: 'Платёж |||| Платежи',
      fields: {
        id: 'ID',
        user_id: 'Пользователь',
        status: 'Статус',
        amountKopecks: 'Сумма (коп.)',
        refundedAmountKopecks: 'Возвращено (коп.)',
        createdAt: 'Создан',
        updatedAt: 'Обновлён',
        succeededAt: 'Успешно оплачен',
        paymentMethodId: 'Способ оплаты',
        'tariff.name': 'Тариф',
        period: 'Период',
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
        occupancy: 'Заполняемость',
        address: 'Адрес',
        description: 'Описание',
        createdAt: 'Создан',
        updatedAt: 'Обновлён',
      },
    },
    leases: {
      name: 'Договор аренды |||| Договоры аренды',
      fields: {
        id: 'ID',
        ownerId: 'Владелец',
        propertyId: 'Объект',
        tenantContactId: 'Контакт арендатора',
        status: 'Статус',
        startDate: 'Дата начала',
        endDate: 'Дата окончания',
        rentAmountKopecks: 'Аренда (коп.)',
        depositAmountKopecks: 'Депозит (коп.)',
        paymentDay: 'День платежа',
        comment: 'Комментарий',
        createdAt: 'Создан',
        updatedAt: 'Обновлён',
      },
    },
    tenantContacts: {
      name: 'Контакт арендатора |||| Контакты арендаторов',
      fields: {
        id: 'ID',
        ownerId: 'Владелец',
        name: 'Имя',
        surname: 'Фамилия',
        patronymic: 'Отчество',
        phone: 'Телефон',
        email: 'Email',
        comment: 'Комментарий',
        createdAt: 'Создан',
        updatedAt: 'Обновлён',
      },
    },
    operations: {
      name: 'Операция |||| Операции',
      fields: {
        id: 'ID',
        ownerId: 'Владелец',
        propertyId: 'Объект',
        leaseId: 'Договор',
        recurringOperationId: 'Периодическая операция',
        type: 'Тип',
        category: 'Категория',
        name: 'Название',
        amountKopecks: 'Сумма (коп.)',
        operationDate: 'Дата операции',
        status: 'Статус',
        comment: 'Комментарий',
        isException: 'Исключение',
        reminderOffsetDays: 'Напомнить за (дней)',
        createdAt: 'Создан',
        updatedAt: 'Обновлён',
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
