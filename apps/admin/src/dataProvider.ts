import {
  type DataProvider,
  HttpError,
  type CreateParams,
  type CreateResult,
  type GetListParams,
  type GetListResult,
  type GetManyParams,
  type GetManyReferenceParams,
  type GetManyReferenceResult,
  type GetManyResult,
  type GetOneParams,
  type GetOneResult,
  type RaRecord,
  type UpdateParams,
  type UpdateResult,
} from 'react-admin';

const API_PREFIX = import.meta.env.VITE_API_PREFIX ?? '/api';

interface ProblemDetails {
  detail?: string;
  title?: string;
}

// Экспортируется для типизации useDataProvider<AdminDataProvider>() в
// компонентах: кастомные методы (refundPayment, getStats, …) доступны на
// значении провайдера, а не на базовом типе DataProvider хука.
export interface AdminDataProvider extends DataProvider {
  // Возврат всегда полный (ADR 0037): контракт эндпоинта не принимает тело.
  refundPayment: (payload: { id: string | number }) => Promise<{ data: unknown }>;
  syncPayment: (payload: { id: string | number }) => Promise<{ data: unknown }>;
  // Админ-операции над подпиской (issue #255).
  assignServiceSubscription: (payload: {
    userId: string | number;
    tariffName: string;
    termType: 'month' | 'year' | 'date';
    untilDate?: string;
  }) => Promise<{ data: unknown }>;
  forceChangeSubscriptionTariff: (payload: {
    userId: string | number;
    tariffName: string;
    period: 'month' | 'year';
  }) => Promise<{ data: unknown }>;
  extendSubscriptionGrace: (payload: { userId: string | number; days: number }) => Promise<{ data: unknown }>;
  cancelSubscription: (payload: { userId: string | number }) => Promise<{ data: unknown }>;
  // Статистика дашборда (issue #307): обращения к backend — только через
  // dataProvider, сырой fetch в компонентах запрещён ESLint-гейтом.
  getStats: () => Promise<{ data: unknown }>;
}

const httpClient = async (url: string, options: RequestInit = {}): Promise<{ json: unknown; headers: Headers }> => {
  const response = await fetch(url, { ...options, credentials: 'include' });

  if (!response.ok) {
    const contentType = response.headers.get('content-type') ?? '';
    let body: ProblemDetails | string | null = null;

    try {
      if (contentType.includes('application/json') || contentType.includes('application/problem+json')) {
        body = (await response.json()) as ProblemDetails;
      } else {
        body = await response.text();
      }
    } catch {
      body = null;
    }

    const message =
      typeof body === 'object' && body
        ? (body.detail ?? body.title ?? response.statusText)
        : typeof body === 'string'
          ? body
          : response.statusText;

    throw new HttpError(message, response.status, body);
  }

  const contentLength = response.headers.get('content-length');
  if (response.status === 204 || contentLength === '0') {
    return { json: null, headers: response.headers };
  }

  const json = (await response.json()) as unknown;
  return { json, headers: response.headers };
};

// Бэкенд отдаёт идентификатор в поле Id (PascalCase) — ensureId переносит
// его в react-admin'овский id. Трюк с аннотацией вместо каста: RaRecord
// индексируется любым значением, прямое чтение дало бы any. Значения не
// string/number игнорируются — строковизация объекта дала бы «[object Object]».
const ensureId = <T extends RaRecord>(item: T): T => {
  const record: Record<string, unknown> = item;
  if (item.id === undefined) {
    const backendId = record.Id;
    if (typeof backendId === 'string' || typeof backendId === 'number') {
      return { ...item, id: String(backendId) };
    }
  }
  return item;
};

// Sortable-поля по ресурсу. Источник истины — whitelist'ы сортировки бэкенда
// (apps/backend/internal/admin/application/service.go,
// apps/backend/internal/billing/application/payment_service.go);
// при расширении бэкендных списков держать мапу синхронно.
// Ресурсы без серверной сортировки (tariffs) объявляются пустым списком.
const sortableFieldsByResource: Record<string, readonly string[]> = {
  users: ['createdAt', 'updatedAt'],
  properties: ['name', 'createdAt', 'updatedAt', 'status'],
  leases: ['startDate', 'updatedAt', 'status', 'rentAmountKopecks'],
  operations: ['operationDate', 'amountKopecks', 'status'],
  tenantContacts: ['name', 'updatedAt'],
  subscriptionPayments: ['createdAt', 'amountKopecks', 'status'],
  auditLogs: ['createdAt'],
  tariffs: [],
};

// Ресурсы без серверной пагинации: эндпоинт отдаёт полный список, limit/offset
// в его контракте не объявлены — не отправляем их, чтобы запрос соответствовал
// OpenAPI-контракту.
const unpaginatedResources = new Set(['tariffs', 'subscriptionTransitions']);

// Фильтры списков приходят из react-admin как any (GetListParams.filter) —
// на границе якорятся в Record<string, unknown> и сужаются по месту.
type ListFilter = Record<string, unknown>;

const buildListQuery = (resource: string, params: GetListParams): string => {
  const { pagination, sort } = params;
  const filter = (params.filter ?? {}) as ListFilter;
  const query = new URLSearchParams();

  if (!unpaginatedResources.has(resource)) {
    const page = pagination?.page ?? 1;
    const perPage = pagination?.perPage ?? 10;

    query.set('limit', String(perPage));
    query.set('offset', String((page - 1) * perPage));
  }

  // Отбрасываем sort/order вне whitelist'а ресурса: бэкенд валидирует sort
  // и отвечает 400, а в URL списков у пользователей могли остаться старые
  // значения (sort=id, sort=phone). Без sort бэкенд применит свой дефолт.
  if (sort?.field && sortableFieldsByResource[resource]?.includes(sort.field)) {
    query.set('sort', sort.field);
    query.set('order', sort.order === 'ASC' ? 'asc' : 'desc');
  }

  // В query-строку уходят только примитивы: String(объекта) дал бы
  // «[object Object]» — значения прочих типов молча отбрасываются.
  Object.entries(filter).forEach(([key, value]) => {
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
      if (value !== '') {
        query.set(key, String(value));
      }
    }
  });

  const qs = query.toString();
  return qs ? `?${qs}` : '';
};

// Без ownerId используются плоские эндпоинты этапа 2, с ownerId — вложенные
// (обратная совместимость с вкладками UserShow).
const listUrl = (resource: string, ownerId?: string | number): string => {
  const hasOwner = ownerId !== undefined && ownerId !== '';
  switch (resource) {
    case 'users':
      return `${API_PREFIX}/admin/users`;
    case 'properties':
      return hasOwner ? `${API_PREFIX}/admin/users/${ownerId}/properties` : `${API_PREFIX}/admin/properties`;
    case 'leases':
      return hasOwner ? `${API_PREFIX}/admin/users/${ownerId}/leases` : `${API_PREFIX}/admin/leases`;
    case 'tenantContacts':
      return hasOwner ? `${API_PREFIX}/admin/users/${ownerId}/tenant-contacts` : `${API_PREFIX}/admin/tenant-contacts`;
    case 'propertyContacts':
      return `${API_PREFIX}/admin/property-contacts`;
    case 'operations':
      return hasOwner ? `${API_PREFIX}/admin/users/${ownerId}/operations` : `${API_PREFIX}/admin/operations`;
    case 'subscriptionPayments':
      return `${API_PREFIX}/admin/subscription/payments`;
    case 'tariffs':
      return `${API_PREFIX}/admin/tariffs`;
    case 'auditLogs':
      return hasOwner ? `${API_PREFIX}/admin/users/${ownerId}/audit-logs` : `${API_PREFIX}/admin/audit-logs`;
    // История переходов подписки существует только в рамках пользователя
    // (issue #255) — плоского эндпоинта нет.
    case 'subscriptionTransitions':
      if (!hasOwner) {
        throw new Error('subscriptionTransitions requires a user id');
      }
      return `${API_PREFIX}/admin/users/${ownerId}/subscription/transitions`;
    default:
      throw new Error(`Unknown resource: ${resource}`);
  }
};

const oneUrl = (resource: string, id: string | number): string => {
  switch (resource) {
    case 'users':
      return `${API_PREFIX}/admin/users/${id}`;
    case 'properties':
      return `${API_PREFIX}/admin/properties/${id}`;
    case 'leases':
      return `${API_PREFIX}/admin/leases/${id}`;
    case 'tenantContacts':
      return `${API_PREFIX}/admin/tenant-contacts/${id}`;
    case 'propertyContacts':
      return `${API_PREFIX}/admin/property-contacts/${id}`;
    case 'operations':
      return `${API_PREFIX}/admin/operations/${id}`;
    case 'subscriptionPayments':
      return `${API_PREFIX}/admin/subscription/payments/${id}`;
    case 'auditLogs':
      return `${API_PREFIX}/admin/audit-logs/${id}`;
    default:
      throw new Error(`Unknown resource: ${resource}`);
  }
};

const parseListResponse = <T extends RaRecord>(json: unknown, headers: Headers): { data: T[]; total: number } => {
  let items: T[] = [];
  let total = 0;

  if (Array.isArray(json)) {
    items = json as T[];
    const headerTotal = headers.get('X-Total-Count');
    total = headerTotal ? Number(headerTotal) : items.length;
  } else if (json && typeof json === 'object') {
    const obj = json as Record<string, unknown>;
    if (Array.isArray(obj.data)) {
      items = obj.data as T[];
      total = typeof obj.total === 'number' ? obj.total : items.length;
    } else if (Array.isArray(obj.items)) {
      items = obj.items as T[];
      total = typeof obj.total === 'number' ? obj.total : items.length;
    }
  }

  return { data: items.map(ensureId), total };
};

export const dataProvider: AdminDataProvider = {
  getList: async <T extends RaRecord>(resource: string, params: GetListParams): Promise<GetListResult<T>> => {
    const ownerId = (params.filter as ListFilter | undefined)?.owner_id as string | number | undefined;
    const url = `${listUrl(resource, ownerId)}${buildListQuery(resource, params)}`;
    const { json, headers } = await httpClient(url, { method: 'GET' });
    return parseListResponse<T>(json, headers);
  },

  getOne: async <T extends RaRecord>(resource: string, params: GetOneParams<T>): Promise<GetOneResult<T>> => {
    const { json } = await httpClient(oneUrl(resource, params.id), { method: 'GET' });

    let data: T;
    if (json && typeof json === 'object') {
      const obj = json as Record<string, unknown>;
      switch (resource) {
        case 'users':
          data = {
            ...(obj.user ?? {}),
            subscription: obj.subscription,
            stats: obj.stats,
          } as unknown as T;
          break;
        case 'properties':
          data = (obj.property ?? json) as T;
          break;
        case 'leases':
          data = (obj.lease ?? json) as T;
          break;
        case 'tenantContacts':
          data = (obj.contact ?? json) as T;
          break;
        case 'operations':
          data = (obj.operation ?? json) as T;
          break;
        case 'auditLogs':
          data = (obj.auditLog ?? json) as T;
          break;
        case 'propertyContacts':
          data = json as T;
          break;
        case 'subscriptionPayments':
        default:
          data = json as T;
          break;
      }
    } else {
      data = json as T;
    }

    return { data: ensureId(data) };
  },

  getMany: async <T extends RaRecord>(resource: string, params: GetManyParams<T>): Promise<GetManyResult<T>> => {
    const results = await Promise.all(params.ids.map((id) => dataProvider.getOne<T>(resource, { id })));
    return { data: results.map((r) => r.data) };
  },

  getManyReference: async <T extends RaRecord>(
    resource: string,
    params: GetManyReferenceParams
  ): Promise<GetManyReferenceResult<T>> => {
    // owner_id — вложенный эндпоинт; property_id/lease_id — плоский эндпоинт с фильтром.
    if (params.target === 'owner_id') {
      const filter: ListFilter = { ...(params.filter ?? {}) as ListFilter, owner_id: params.id };
      const url = `${listUrl(resource, params.id)}${buildListQuery(resource, { ...params, filter })}`;
      const { json, headers } = await httpClient(url, { method: 'GET' });
      return parseListResponse<T>(json, headers);
    }

    if (params.target === 'property_id' || params.target === 'lease_id') {
      const filter: ListFilter = { ...(params.filter ?? {}) as ListFilter, [params.target]: params.id };
      const url = `${listUrl(resource)}${buildListQuery(resource, { ...params, filter })}`;
      const { json, headers } = await httpClient(url, { method: 'GET' });
      return parseListResponse<T>(json, headers);
    }

    throw new HttpError(`getManyReference для ${resource} не поддерживает target=${params.target}`, 400);
  },

  create: async <T extends RaRecord>(resource: string, params: CreateParams<T>): Promise<CreateResult<T>> => {
    if (resource !== 'tariffs') {
      throw new HttpError(`Создание для ресурса ${resource} не поддерживается`, 405);
    }
    const { json } = await httpClient(`${API_PREFIX}/admin/tariffs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params.data),
    });
    return { data: ensureId(json as T) };
  },

  update: async <T extends RaRecord>(resource: string, params: UpdateParams<T>): Promise<UpdateResult<T>> => {
    if (resource !== 'tariffs') {
      throw new HttpError(`Обновление для ресурса ${resource} не поддерживается`, 405);
    }
    // PUT /admin/tariffs/{id} принимает полное редактируемое состояние. Вызов
    // может передать только часть (скрытие меняет один isActive) — недостающие
    // поля догружаются из previousData; всё вне контракта (id, name) не отправляется.
    const prev = params.previousData as Record<string, unknown> | undefined;
    const data = params.data as Record<string, unknown>;
    const editableFields = ['activePropertyLimit', 'monthlyPriceKopecks', 'yearlyPriceKopecks', 'isActive'] as const;
    const body: Record<string, unknown> = {};
    for (const field of editableFields) {
      body[field] = data[field] !== undefined ? data[field] : prev?.[field];
    }
    const { json } = await httpClient(`${API_PREFIX}/admin/tariffs/${params.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    return { data: ensureId(json as T) };
  },

  // Неподдерживаемые методы отдают rejected promise (Promise<never>
  // присваивается Promise-сигнатуре интерфейса) — async/throw без await
  // ловился бы require-await.
  updateMany: (resource: string): Promise<never> =>
    Promise.reject(new HttpError(`Массовое обновление для ресурса ${resource} не поддерживается`, 405)),

  delete: (resource: string): Promise<never> =>
    Promise.reject(new HttpError(`Удаление для ресурса ${resource} не поддерживается`, 405)),

  deleteMany: (resource: string): Promise<never> =>
    Promise.reject(new HttpError(`Массовое удаление для ресурса ${resource} не поддерживается`, 405)),

  refundPayment: async ({ id }) => {
    const { json } = await httpClient(`${API_PREFIX}/admin/subscription/payments/${id}/refund`, { method: 'POST' });
    return { data: json };
  },

  syncPayment: async ({ id }) => {
    const { json } = await httpClient(`${API_PREFIX}/admin/subscription/payments/${id}/sync`, { method: 'POST' });
    return { data: json };
  },

  assignServiceSubscription: async ({ userId, tariffName, termType, untilDate }) => {
    const body: Record<string, string> = { tariffName, termType };
    if (termType === 'date' && untilDate) {
      body.untilDate = untilDate;
    }
    const { json } = await httpClient(`${API_PREFIX}/admin/users/${userId}/subscription/service`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    return { data: json };
  },

  forceChangeSubscriptionTariff: async ({ userId, tariffName, period }) => {
    const { json } = await httpClient(`${API_PREFIX}/admin/users/${userId}/subscription/force-change`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tariffName, period }),
    });
    return { data: json };
  },

  extendSubscriptionGrace: async ({ userId, days }) => {
    const { json } = await httpClient(`${API_PREFIX}/admin/users/${userId}/subscription/grace-extension`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ days }),
    });
    return { data: json };
  },

  cancelSubscription: async ({ userId }) => {
    const { json } = await httpClient(`${API_PREFIX}/admin/users/${userId}/subscription/cancel`, { method: 'POST' });
    return { data: json };
  },

  getStats: async () => {
    const { json } = await httpClient(`${API_PREFIX}/admin/stats`, { method: 'GET' });
    return { data: json };
  },
};
