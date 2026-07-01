import { AuthProvider, fetchUtils, HttpError } from 'react-admin';

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

const fetchMe = async (): Promise<MeResponse> => {
  const { json } = await httpClient(`${API_PREFIX}/me`, { method: 'GET' });
  return json as MeResponse;
};

export const authProvider: AuthProvider = {
  login: async ({ phone, email, code }) => {
    await httpClient(`${API_PREFIX}/auth/email/verify`, {
      method: 'POST',
      body: JSON.stringify({ phone, email, code }),
    });

    const me = await fetchMe();
    if (me.role !== 'admin') {
      await httpClient(`${API_PREFIX}/auth/logout`, { method: 'POST' });
      throw new HttpError('Доступ только для администраторов', 403);
    }
  },

  checkAuth: async () => {
    const me = await fetchMe();
    if (me.role !== 'admin') {
      throw new HttpError('Доступ только для администраторов', 403);
    }
  },

  checkError: async (error) => {
    const status = (error as HttpError | undefined)?.status;
    if (status === 401 || status === 403) {
      throw error;
    }
  },

  logout: async () => {
    await httpClient(`${API_PREFIX}/auth/logout`, { method: 'POST' });
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
