'use client';

import { useMe } from '../api/hooks';
import { ROUTES } from '@/shared/config/routes';

export function useAuthLanding() {
    const { data, isPending } = useMe();
    const isAuthenticated = Boolean(data);
    return {
        isAuthenticated,
        isLoading: isPending,
        ctaHref: isAuthenticated ? ROUTES.dashboard : ROUTES.login,
    };
}
