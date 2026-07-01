import {
  DataProvider,
  HttpError,
  type CreateParams,
  type CreateResult,
  type DeleteManyParams,
  type DeleteManyResult,
  type DeleteParams,
  type DeleteResult,
  type GetListParams,
  type GetListResult,
  type GetManyParams,
  type GetManyReferenceParams,
  type GetManyReferenceResult,
  type GetManyResult,
  type GetOneParams,
  type GetOneResult,
  type RaRecord,
  type UpdateManyParams,
  type UpdateManyResult,
  type UpdateParams,
  type UpdateResult,
} from 'react-admin';

const API_PREFIX = import.meta.env.VITE_API_PREFIX || '/api';

interface ProblemDetails {
  detail?: string;
  title?: string;
}

interface AdminDataProvider extends DataProvider {
  refundPayment: (payload: { id: string | number; amountKopecks?: number }) => Promise<{ data: unknown }>;
  syncPayment: (payload: { id: string | number }) => Promise<{ data: unknown }>;
}

const httpClient = async (url: string, options: RequestInit = {}): Promise<{ json: unknown; headers: Headers }> => {
  const response = await fetch(url, { ...options, credentials: 'include' });

  if (!response.ok) {
    const contentType = response.headers.get('content-type') || '';
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
      (typeof body === 'object' && body && (body.detail || body.title)) ||
      (typeof body === 'string' ? body : response.statusText);

    throw new HttpError(message, response.status, body);
  }

  const contentLength = response.headers.get('content-length');
  if (response.status === 204 || contentLength === '0') {
    return { json: null, headers: response.headers };
  }

  const json = (await response.json()) as unknown;
  return { json, headers: response.headers };
};

const ensureId = <T extends RaRecord>(item: T): T => {
  if (item.id === undefined && (item as Record<string, unknown>).Id !== undefined) {
    return { ...item, id: String((item as Record<string, unknown>).Id) } as T;
  }
  return item;
};

const buildListQuery = (params: GetListParams): string => {
  const { pagination, sort, filter } = params;
  const query = new URLSearchParams();

  const page = pagination?.page ?? 1;
  const perPage = pagination?.perPage ?? 10;

  query.set('limit', String(perPage));
  query.set('offset', String((page - 1) * perPage));

  if (sort?.field) {
    query.set('sort', sort.field);
    query.set('order', sort.order === 'ASC' ? 'asc' : 'desc');
  }

  Object.entries(filter || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value));
    }
  });

  const qs = query.toString();
  return qs ? `?${qs}` : '';
};

const listUrl = (resource: string, ownerId?: string | number): string => {
  switch (resource) {
    case 'users':
      return `${API_PREFIX}/admin/users`;
    case 'properties':
      return `${API_PREFIX}/admin/users/${ownerId}/properties`;
    case 'leases':
      return `${API_PREFIX}/admin/users/${ownerId}/leases`;
    case 'tenantContacts':
      return `${API_PREFIX}/admin/users/${ownerId}/tenant-contacts`;
    case 'operations':
      return `${API_PREFIX}/admin/users/${ownerId}/operations`;
    case 'subscriptionPayments':
      return `${API_PREFIX}/admin/subscription/payments`;
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
    case 'operations':
      return `${API_PREFIX}/admin/operations/${id}`;
    case 'subscriptionPayments':
      return `${API_PREFIX}/admin/subscription/payments/${id}`;
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
    if (['properties', 'leases', 'tenantContacts', 'operations'].includes(resource) && !params.filter?.owner_id) {
      throw new HttpError(`Ресурс ${resource} требует фильтр owner_id`, 400);
    }

    const url = `${listUrl(resource, params.filter?.owner_id as string | number)}${buildListQuery(params)}`;
    const { json, headers } = await httpClient(url, { method: 'GET' });
    return parseListResponse<T>(json, headers);
  },

  getOne: async <T extends RaRecord>(resource: string, params: GetOneParams): Promise<GetOneResult<T>> => {
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

  getMany: async <T extends RaRecord>(resource: string, params: GetManyParams): Promise<GetManyResult<T>> => {
    const results = await Promise.all(params.ids.map((id) => dataProvider.getOne<T>(resource, { id })));
    return { data: results.map((r) => r.data) };
  },

  getManyReference: async <T extends RaRecord>(
    resource: string,
    params: GetManyReferenceParams
  ): Promise<GetManyReferenceResult<T>> => {
    if (params.target !== 'owner_id') {
      throw new HttpError(`getManyReference для ${resource} поддерживает только target=owner_id`, 400);
    }

    const url = `${listUrl(resource, params.id)}${buildListQuery({ ...params, filter: { ...params.filter, owner_id: params.id } })}`;
    const { json, headers } = await httpClient(url, { method: 'GET' });
    return parseListResponse<T>(json, headers);
  },

  create: async <T extends RaRecord>(resource: string, _params: CreateParams<T>): Promise<CreateResult<T>> => {
    throw new HttpError(`Создание для ресурса ${resource} не поддерживается`, 405);
  },

  update: async <T extends RaRecord>(resource: string, _params: UpdateParams<T>): Promise<UpdateResult<T>> => {
    throw new HttpError(`Обновление для ресурса ${resource} не поддерживается`, 405);
  },

  updateMany: async <T extends RaRecord>(resource: string, _params: UpdateManyParams<T>): Promise<UpdateManyResult<T>> => {
    throw new HttpError(`Массовое обновление для ресурса ${resource} не поддерживается`, 405);
  },

  delete: async <T extends RaRecord>(resource: string, _params: DeleteParams<T>): Promise<DeleteResult<T>> => {
    throw new HttpError(`Удаление для ресурса ${resource} не поддерживается`, 405);
  },

  deleteMany: async <T extends RaRecord>(resource: string, _params: DeleteManyParams<T>): Promise<DeleteManyResult<T>> => {
    throw new HttpError(`Массовое удаление для ресурса ${resource} не поддерживается`, 405);
  },

  refundPayment: async ({ id, amountKopecks }) => {
    const body = amountKopecks !== undefined ? JSON.stringify({ amount_kopecks: amountKopecks }) : undefined;
    const { json } = await httpClient(`${API_PREFIX}/admin/subscription/payments/${id}/refund`, {
      method: 'POST',
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body,
    });
    return { data: json };
  },

  syncPayment: async ({ id }) => {
    const { json } = await httpClient(`${API_PREFIX}/admin/subscription/payments/${id}/sync`, { method: 'POST' });
    return { data: json };
  },
};
