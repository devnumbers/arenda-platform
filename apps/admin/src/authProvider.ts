import { AuthProvider, fetchUtils } from 'react-admin';
import { normalizePhone } from './phone';

const API_PREFIX = import.meta.env.VITE_API_PREFIX || '/api';

interface MeResponse {
  id: string;
  role: string;
  email?: string;
  full_name?: string;
  phone?: string;
}

const httpClient = (url: string, options: RequestInit = {}) =>
  fetchUtils.fetchJson(url, { ...options, credentials: 'include' });

interface ProblemDetails {
  detail?: string;
  title?: string;
}

const statusOf = (error: unknown): number | undefined => (error as { status?: number } | undefined)?.status;

const messageOf = (error: unknown, fallback: string): string => {
  const body = (error as { body?: ProblemDetails | string | null } | undefined)?.body;
  if (body && typeof body === 'object') {
    return body.detail || body.title || fallback;
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
  sendLoginCode: (payload: { phone: string; email: string }) => Promise<void>;
}

export const authProvider: AdminAuthProvider = {
  sendLoginCode: async ({ phone, email }) => {
    try {
      await httpClient(`${API_PREFIX}/auth/send`, {
        method: 'POST',
        body: JSON.stringify({ phone, email }),
      });
    } catch (error) {
      throw new Error(messageOf(error, 'Не удалось отправить код'));
    }
  },

  login: async ({ phone, email, code }) => {
    const normalizedPhone = normalizePhone(String(phone ?? ''));
    try {
      await httpClient(`${API_PREFIX}/auth/verify`, {
        method: 'POST',
        body: JSON.stringify({ phone: normalizedPhone, email, code }),
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

  checkError: async (error) => {
    const status = statusOf(error);
    if (status === 401 || status === 403) {
      throw silentLoginRedirect();
    }
  },

  logout: async () => {
    await logoutSession();
  },

  getIdentity: async () => {
    const me = await fetchMe();
    return {
      id: me.id,
      fullName: me.full_name || me.email || me.phone || me.id,
      avatar: undefined,
    };
  },

  getPermissions: async () => {
    const me = await fetchMe();
    return me.role;
  },
};
