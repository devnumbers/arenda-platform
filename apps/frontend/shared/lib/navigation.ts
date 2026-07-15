import type {useRouter} from 'next/navigation';

type AppRouter = ReturnType<typeof useRouter>;

export const RETURN_TO_PARAM = 'returnTo';

/** Allow only internal absolute paths; reject protocol-relative, scheme and backslash tricks. */
export function sanitizeReturnTo(value: string | undefined): string | undefined {
    if (!value || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) {
        return undefined;
    }
    return value;
}

/** Merge params into the returnTo query string; existing query params are preserved. */
export function buildReturnUrl(returnTo: string, params: Record<string, string>): string {
    const [path, query] = returnTo.split('?');
    const search = new URLSearchParams(query);
    for (const [key, value] of Object.entries(params)) {
        search.set(key, value);
    }
    const qs = search.toString();
    return qs ? `${path}?${qs}` : path;
}

export function goBack(router: AppRouter, fallbackHref: string): void {
    if (typeof window !== 'undefined' && window.history.length > 1) {
        router.back();
    } else {
        router.replace(fallbackHref);
    }
}
