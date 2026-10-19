import { type AuthProvider, fetchUtils } from 'react-admin';

const API_PREFIX = import.meta.env.VITE_API_PREFIX ?? '/api';

interface MeResponse {
  id: string;
  role: string;
  email?: string;
  full_name?: string;
}

// Граница типизирована (админ-трек II, #392): fetchUtils.fetchJson отдаёт
// json как any — обёртка переводит его в unknown до первого использования,
// дальше полезная нагрузка сужается целевыми типами (MeResponse).
const httpClient = async (
  url: string,
  options: RequestInit = {}
): Promise<{ json: unknown; headers: Headers }> => {
  const response = await fetchUtils.fetchJson(url, { ...options, credentials: 'include' });
  return { json: response.json as unknown, headers: response.headers };
};

interface ProblemDetails {
  detail?: string;
  title?: string;
}

const statusOf = (error: unknown): number | undefined => (error as { status?: number } | undefined)?.status;

const messageOf = (error: unknown, fallback: string): string => {
  const body = (error as { body?: ProblemDetails | string | null } | undefined)?.body;
  if (body && typeof body === 'object') {
    return body.detail ?? body.title ?? fallback;
  }
  if (typeof body === 'string' && body) {
    return body;
  }
  return error instanceof Error && error.message ? error.message : fallback;
};

const silentLoginRedirect = (): Error => {
  const error = new Error() as Error & { redirectTo?: string };
  (error as unknown as { message: false }).message = false;
  error.redirectTo = '/login';
  return error;
};

const logoutSession = async (): Promise<void> => {
  try {
    await httpClient(`${API_PREFIX}/auth/logout`, { method: 'POST' });
  } catch (error) {
    if (statusOf(error) !== 401) {
      throw error;
    }
  }
};

const fetchMe = async (): Promise<MeResponse> => {
  const { json } = await httpClient(`${API_PREFIX}/me`, { method: 'GET' });
  return json as MeResponse;
};

// HTTP аутентификации живёт только в authProvider (ESLint no-restricted-globals
// на fetch вне граничных файлов): LoginPage не ходит в сеть сам.
interface AdminAuthProvider extends AuthProvider {
  /** Шаг «отправить код подтверждения» экрана входа (POST /auth/send). */
  sendLoginCode: (payload: { email: string }) => Promise<void>;
}

// Полезная нагрузка POST /auth/verify из экрана входа: интерфейс AuthProvider
// типизирует login(params: any) — граница сужает параметры сама.
// Вход = регистрация (ADR 0067): адрес канонизирует бэкенд, граница
// отправляет только обрезанное значение — оба шага (send и verify) одинаково.
interface LoginParams {
  email?: string;
  code?: string;
}

export const authProvider: AdminAuthProvider = {
  sendLoginCode: async ({ email }) => {
    try {
      await httpClient(`${API_PREFIX}/auth/send`, {
        method: 'POST',
        body: JSON.stringify({ email: String(email ?? '').trim() }),
      });
    } catch (error) {
      throw new Error(messageOf(error, 'Не удалось отправить код'));
    }
  },

  login: async ({ email, code }: LoginParams) => {
    const trimmedEmail = String(email ?? '').trim();
    try {
      await httpClient(`${API_PREFIX}/auth/verify`, {
        method: 'POST',
        body: JSON.stringify({ email: trimmedEmail, code }),
      });
    } catch (error) {
      throw new Error(messageOf(error, 'Не удалось подтвердить код'));
    }

    let me: MeResponse;
    try {
      me = await fetchMe();
    } catch (error) {
      if (statusOf(error) === 401) {
        throw new Error('Сессия не была сохранена. Обновите страницу и попробуйте ещё раз.');
      }
      throw new Error(messageOf(error, 'Не удалось проверить права администратора'));
    }
    if (me.role !== 'admin') {
      await logoutSession();
      throw new Error('Доступ только для администраторов');
    }
  },

  checkAuth: async () => {
    let me: MeResponse;
    try {
      me = await fetchMe();
    } catch (error) {
      if (statusOf(error) === 401) {
        throw silentLoginRedirect();
      }
      throw error;
    }
    if (me.role !== 'admin') {
      throw new Error('Доступ только для администраторов');
    }
  },

  // Без await: решение по статусу синхронно, редирект — rejected promise
  // (требование типа Promise<void> у AuthProvider).
  checkError: (error: unknown) => {
    const status = statusOf(error);
    if (status === 401 || status === 403) {
      return Promise.reject(silentLoginRedirect());
    }
    return Promise.resolve();
  },

  logout: async () => {
    await logoutSession();
  },

  getIdentity: async () => {
    const me = await fetchMe();
    return {
      id: me.id,
      fullName: me.full_name ?? me.email ?? me.id,
      avatar: undefined,
    };
  },

  getPermissions: async () => {
    const me = await fetchMe();
    return me.role;
  },
};
