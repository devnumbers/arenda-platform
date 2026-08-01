import { ApiError } from '@/shared/api/errors';
import { ROUTES } from '@/shared/config/routes';
import { notify } from './base';
import type {
  ErrorScenarioFn,
  NotificationKey,
  PromiseScenarioFn,
  ScenarioFn,
  ScenarioOptions,
} from './types';

function getApiErrorDetail(error: unknown): string | undefined {
  if (error instanceof ApiError) {
    return error.detail;
  }
  return undefined;
}

function errorScenario(fallbackTitle: string): ErrorScenarioFn {
  return (error, options) =>
    notify.error(getApiErrorDetail(error) ?? fallbackTitle, options);
}

type PromiseMessages = {
  loading: string;
  success: string;
  errorFallback: string;
};

function runPromiseScenario<T>(
  promise: Promise<T>,
  messages: PromiseMessages,
  options?: ScenarioOptions,
): NotificationKey {
  const loadingKey = notify.loading(messages.loading, {
    description: options?.description,
  });

  promise
    .then(() => {
      notify.close(loadingKey);
      notify.success(messages.success, options);
    })
    .catch((error: unknown) => {
      notify.close(loadingKey);
      notify.error(
        (error as ApiError).detail ?? messages.errorFallback,
        options,
      );
    });

  return loadingKey;
}

const auth = {
  loginError: ((options?) =>
    notify.error('Не удалось войти', options)) satisfies ScenarioFn,
  addCardStartError: ((options?) =>
    notify.error('Не удалось начать добавление карты', options)) satisfies ScenarioFn,
} as const;

const tenants = {
  tenantUpdated: ((options?) =>
    notify.success('Арендатор обновлён', options)) satisfies ScenarioFn,
  tenantUpdateError: errorScenario('Не удалось обновить арендатора'),
  tenantCreateError: errorScenario('Не удалось создать арендатора'),
} as const;

const propertyContacts = {
  created: ((options?) =>
    notify.success('Контакт добавлен', options)) satisfies ScenarioFn,
  createError: errorScenario('Не удалось добавить контакт'),
  updated: ((options?) =>
    notify.success('Контакт обновлён', options)) satisfies ScenarioFn,
  updateError: errorScenario('Не удалось обновить контакт'),
  deleted: ((options?) =>
    notify.success('Контакт удалён', options)) satisfies ScenarioFn,
  deleteError: errorScenario('Не удалось удалить контакт'),
} as const;

const property = {
  detailError: ((options?) =>
    notify.error('Не удалось загрузить объект', options)) satisfies ScenarioFn,
  movedToMaintenance: ((options?) =>
    notify.success('Объект переведён на ремонт', options)) satisfies ScenarioFn,
  returnedToWork: ((options?) =>
    notify.success('Объект возвращён в работу', options)) satisfies ScenarioFn,
  returnedFromArchive: ((options?) =>
    notify.success('Объект возвращён из архива', options)) satisfies ScenarioFn,
  movedToArchive: ((options?) =>
    notify.success('Объект переведён в архив', {
      action: {label: 'В архив', href: ROUTES.propertyArchive},
      ...options,
    })) satisfies ScenarioFn,
  updated: ((options?) =>
    notify.success('Объект обновлён', options)) satisfies ScenarioFn,
  saveError: ((options?) =>
    notify.error('Не удалось сохранить изменения', options)) satisfies ScenarioFn,
  listLoadError: ((options?) =>
    notify.error('Не удалось загрузить список объектов', options)) satisfies ScenarioFn,
  deleted: ((options?) =>
    notify.success('Объект удалён', options)) satisfies ScenarioFn,
  deleteError: ((options?) =>
    notify.error('Не удалось удалить объект', options)) satisfies ScenarioFn,
  exportError: ((options?) =>
    notify.error('Не удалось сформировать экспорт', options)) satisfies ScenarioFn,
} as const;

const leases = {
  completed: (<T>(promise: Promise<T>, options?: ScenarioOptions): NotificationKey =>
    runPromiseScenario(promise, {
      loading: 'Завершаем аренду...',
      success: 'Аренда завершена',
      errorFallback: 'Не удалось завершить аренду',
    }, options)) satisfies PromiseScenarioFn,
  updated: ((options?) =>
    notify.success('Аренда обновлена', options)) satisfies ScenarioFn,
  leaseCreateError: errorScenario('Не удалось создать аренду'),
  leaseSaveError: errorScenario('Не удалось сохранить аренду'),
} as const;

const operations = {
  categoryCreated: ((options?) =>
    notify.success('Категория создана', options)) satisfies ScenarioFn,
  categoryAlreadyExists: ((options?) =>
    notify.success(
      'Категория уже существует — выбрана существующая',
      options,
    )) satisfies ScenarioFn,
  categoryCreateError: ((options?) =>
    notify.error('Не удалось создать категорию', options)) satisfies ScenarioFn,
  operationSaveError: errorScenario('Не удалось сохранить операцию'),
  operationCreateError: errorScenario('Не удалось создать операцию'),
  recurringOperationSaveError: errorScenario('Не удалось сохранить регулярную операцию'),
  recurringOperationCreateError: errorScenario('Не удалось создать регулярную операцию'),
  recurringOperationDeleted: (<T>(promise: Promise<T>, options?: ScenarioOptions): NotificationKey =>
    runPromiseScenario(promise, {
      loading: 'Удаляем серию...',
      success: 'Серия удалена',
      errorFallback: 'Не удалось удалить серию',
    }, options)) satisfies PromiseScenarioFn,
} as const;

const profile = {
  overviewError: ((options?) =>
    notify.error('Не удалось загрузить профиль', options)) satisfies ScenarioFn,
  personalDataSaved: ((options?) =>
    notify.success('Данные сохранены', options)) satisfies ScenarioFn,
  phoneChanged: ((options?) =>
    notify.success('Номер телефона изменён', options)) satisfies ScenarioFn,
  personalDataSaveError: errorScenario('Не удалось сохранить данные'),
  notificationPreferencesSaveError: errorScenario(
    'Не удалось сохранить настройки уведомлений',
  ),
  phoneSendCodeError: errorScenario('Не удалось отправить код'),
  phoneChangeError: errorScenario('Не удалось изменить номер телефона'),
  logoutError: errorScenario('Не удалось выйти'),
} as const;

const tariff = {
  changeLoading: ((options?) =>
    notify.loading('Меняем тариф...', options)) satisfies ScenarioFn,
  changed: ((options?) =>
    notify.success('Изменение тарифа запланировано', options)) satisfies ScenarioFn,
  changeError: ((options?) =>
    notify.error('Не удалось сменить тариф', options)) satisfies ScenarioFn,
  subscriptionCanceled: (<T>(promise: Promise<T>, options?: ScenarioOptions): NotificationKey =>
    runPromiseScenario(promise, {
      loading: 'Отменяем подписку...',
      success: 'Подписка отменена',
      errorFallback: 'Не удалось отменить подписку',
    }, options)) satisfies PromiseScenarioFn,
} as const;

const paymentMethods = {
  cardAdded: ((options?) =>
    notify.success('Карта успешно добавлена', options)) satisfies ScenarioFn,
  cardAddError: ((options?) =>
    notify.error(
      'Не удалось добавить карту. Попробуйте снова.',
      options,
    )) satisfies ScenarioFn,
  listUpdateWarning: ((options?) =>
    notify.warning('Не удалось обновить список карт', options)) satisfies ScenarioFn,
  activated: (<T>(promise: Promise<T>, options?: ScenarioOptions): NotificationKey =>
    runPromiseScenario(promise, {
      loading: 'Делаем карту основной...',
      success: 'Карта стала основной',
      errorFallback: 'Не удалось сделать карту основной',
    }, options)) satisfies PromiseScenarioFn,
  removed: (<T>(promise: Promise<T>, options?: ScenarioOptions): NotificationKey =>
    runPromiseScenario(promise, {
      loading: 'Удаляем карту...',
      success: 'Карта удалена',
      errorFallback: 'Не удалось удалить карту',
    }, options)) satisfies PromiseScenarioFn,
} as const;

const demo = {
  success: ((options?) =>
    notify.success('Успех', options)) satisfies ScenarioFn,
  error: ((options?) =>
    notify.error('Ошибка', options)) satisfies ScenarioFn,
  info: ((options?) =>
    notify.info('Инфо', options)) satisfies ScenarioFn,
  warning: ((options?) =>
    notify.warning('Внимание', options)) satisfies ScenarioFn,
  promise: (<T>(promise: Promise<T>, options?: ScenarioOptions): NotificationKey =>
    runPromiseScenario(promise, {
      loading: 'Загрузка...',
      success: 'Готово',
      errorFallback: 'Ошибка',
    }, options)) satisfies PromiseScenarioFn,
} as const;

export type Scenarios = {
  readonly auth: typeof auth;
  readonly tenants: typeof tenants;
  readonly propertyContacts: typeof propertyContacts;
  readonly property: typeof property;
  readonly leases: typeof leases;
  readonly operations: typeof operations;
  readonly profile: typeof profile;
  readonly tariff: typeof tariff;
  readonly paymentMethods: typeof paymentMethods;
  readonly demo: typeof demo;
};

export const scenarios: Scenarios = {
  auth,
  tenants,
  propertyContacts,
  property,
  leases,
  operations,
  profile,
  tariff,
  paymentMethods,
  demo,
};
