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
        getApiErrorDetail(error) ?? messages.errorFallback,
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

const access = {
  memberAdded: ((options?) =>
    notify.success('Участник добавлен', options)) satisfies ScenarioFn,
  invited: ((options?) =>
    notify.success('Приглашение отправлено', options)) satisfies ScenarioFn,
  inviteError: errorScenario('Не удалось отправить приглашение'),
  roleChanged: ((options?) =>
    notify.success('Роль изменена', options)) satisfies ScenarioFn,
  roleChangeError: errorScenario('Не удалось изменить роль'),
  revoked: ((options?) =>
    notify.success('Доступ отозван', options)) satisfies ScenarioFn,
  revokeError: errorScenario('Не удалось отозвать доступ'),
  invitationResent: ((options?) =>
    notify.success('Приглашение отправлено повторно', options)) satisfies ScenarioFn,
  resendError: errorScenario('Не удалось переотправить приглашение'),
  invitationCancelled: ((options?) =>
    notify.success('Приглашение отменено', options)) satisfies ScenarioFn,
  cancelInvitationError: errorScenario('Не удалось отменить приглашение'),
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
} as const;

const profile = {
  overviewError: ((options?) =>
    notify.error('Не удалось загрузить профиль', options)) satisfies ScenarioFn,
  personalDataSaved: ((options?) =>
    notify.success('Данные сохранены', options)) satisfies ScenarioFn,
  personalDataSaveError: errorScenario('Не удалось сохранить данные'),
  notificationPreferencesSaveError: errorScenario(
    'Не удалось сохранить настройки уведомлений',
  ),
  phoneSendCodeError: errorScenario('Не удалось отправить код'),
  phoneChangeError: errorScenario('Не удалось изменить номер телефона'),
  logoutError: errorScenario('Не удалось выйти'),
  pushEnabled: ((options?) =>
    notify.success('Пуши включены', options)) satisfies ScenarioFn,
  pushEnableError: errorScenario('Не удалось включить пуши'),
  pushPermissionDenied: ((options?) =>
    notify.info(
      'Уведомления отключены в браузере. Включить можно в настройках сайта.',
      options,
    )) satisfies ScenarioFn,
  pushIosNeedsInstall: ((options?) =>
    notify.info(
      'На iPhone для пушей добавьте приложение на экран «Домой» через Поделиться в Safari.',
      options,
    )) satisfies ScenarioFn,
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
  gracePaymentError: ((options?) =>
    notify.error('Не удалось открыть оплату', options)) satisfies ScenarioFn,
  resumeError: ((options?) =>
    notify.error('Не удалось возобновить тариф', options)) satisfies ScenarioFn,
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

/** Контекст «Платежи» (спека #453, история 52): тосты мутаций. Тексты
 * паузы/возобновления/избранного — из резолюции #452. */
const payments = {
  created: ((options?) =>
    notify.success('Платеж создан', options)) satisfies ScenarioFn,
  createError: errorScenario('Не удалось создать платеж'),
  operationCreated: ((options?) =>
    notify.success('Операция добавлена', options)) satisfies ScenarioFn,
  operationCreateError: errorScenario('Не удалось добавить операцию'),
  updated: ((options?) =>
    notify.success('Изменения сохранены', options)) satisfies ScenarioFn,
  updateError: errorScenario('Не удалось сохранить изменения'),
  deleted: ((options?) =>
    notify.success('Платеж удален', options)) satisfies ScenarioFn,
  deleteError: errorScenario('Не удалось удалить платеж'),
  paused: ((options?) =>
    notify.success('Платеж поставлен на паузу', options)) satisfies ScenarioFn,
  pauseError: errorScenario('Не удалось поставить платеж на паузу'),
  resumed: ((options?) =>
    notify.success('Платеж возобновлен', options)) satisfies ScenarioFn,
  resumeError: errorScenario('Не удалось возобновить платеж'),
  paid: ((options?) =>
    notify.success('Оплата отмечена', options)) satisfies ScenarioFn,
  payError: errorScenario('Не удалось отметить оплату'),
  operationDeleted: ((options?) =>
    notify.success('Операция удалена', options)) satisfies ScenarioFn,
  operationDeleteError: errorScenario('Не удалось удалить операцию'),
  favoriteAdded: ((options?) =>
    notify.success('Платеж добавлен в избранное', options)) satisfies ScenarioFn,
  favoriteRemoved: ((options?) =>
    notify.success('Платеж больше не в избранном', options)) satisfies ScenarioFn,
  favoriteError: errorScenario('Не удалось обновить избранное'),
} as const;

/** Контекст «Аренда» (#530): успех дублирует экран успеха визарда — как у
 * платежей; ошибка создания (вторая незавершённая аренда — 409 с detail).
 * Оплата аренды идёт со страницы операции — её тосты в контексте операций.
 * Правка условий (#532) — формулировки как у правки платежа. */
const rentals = {
  created: ((options?) =>
    notify.success('Аренда создана', options)) satisfies ScenarioFn,
  createError: errorScenario('Не удалось создать аренду'),
  updated: ((options?) =>
    notify.success('Изменения сохранены', options)) satisfies ScenarioFn,
  updateError: errorScenario('Не удалось сохранить изменения'),
  /** Завершение (#534): успех — полноэкранный финал мастера, тост только
   * на ошибку мутации. */
  completeError: errorScenario('Не удалось завершить аренду'),
} as const;

/** Контекст «Задачи» (#499): тосты ошибок мутаций; выполнение/снятие/
 * удаление проходят без уведомлений (решение владельца 2026-09-03: задача
 * просто переезжает между секциями). Исключение — сохранение правки
 * (#502, требование тикета): сообщаем, что изменения коснутся будущих
 * задач, а история останется (формат подсказки — решить на приёмке). */
const tasks = {
  completeError: errorScenario('Не удалось выполнить задачу'),
  uncompleteError: errorScenario('Не удалось снять выполнение'),
  deleteCompletedError: errorScenario('Не удалось удалить выполненные задачи'),
  createError: errorScenario('Не удалось создать задачу'),
  updated: ((options?) =>
    notify.success('Задача сохранена', {
      description: 'Изменения коснутся будущих задач — история останется',
      ...options,
    })) satisfies ScenarioFn,
  updateError: errorScenario('Не удалось сохранить задачу'),
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
  readonly propertyContacts: typeof propertyContacts;
  readonly property: typeof property;
  readonly access: typeof access;
  readonly profile: typeof profile;
  readonly tariff: typeof tariff;
  readonly paymentMethods: typeof paymentMethods;
  readonly payments: typeof payments;
  readonly rentals: typeof rentals;
  readonly tasks: typeof tasks;
  readonly demo: typeof demo;
};

export const scenarios: Scenarios = {
  auth,
  propertyContacts,
  property,
  access,
  profile,
  tariff,
  paymentMethods,
  payments,
  rentals,
  tasks,
  demo,
};
