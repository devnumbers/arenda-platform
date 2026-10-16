import type {useRouter} from 'next/navigation';

type AppRouter = ReturnType<typeof useRouter>;

/** Allow only internal absolute paths; reject protocol-relative, scheme and backslash tricks. */
export function sanitizeReturnTo(value: string | undefined): string | undefined {
    if (!value || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) {
        return undefined;
    }
    return value;
}

/**
 * Merge params into the returnTo query string; existing query params are preserved.
 * Values in params and in the returnTo query must be percent-encoded by the caller:
 * a literal `?` inside a value is not supported and would be silently truncated.
 */
export function buildReturnUrl(returnTo: string, params: Record<string, string>): string {
    const [path = returnTo, query] = returnTo.split('?');
    const search = new URLSearchParams(query);
    for (const [key, value] of Object.entries(params)) {
        search.set(key, value);
    }
    const qs = search.toString();
    return qs ? `${path}?${qs}` : path;
}

export function goBack(router: AppRouter, fallbackHref: string): void {
    if (typeof window !== 'undefined' && window.history.length > 1) {
        // Native history.back() instead of router.back(): Next App Router's router.back()
        // can be a no-op right after router.replace(). The router listens to popstate,
        // so the rendered result is equivalent.
        window.history.back();
    } else {
        router.replace(fallbackHref);
    }
}

/**
 * Полная перезагрузка с заменой текущего entry истории (канон auth-границы,
 * #1098; исключение из router.replace зарегистрировано в CODING_STANDARDS
 * «Navigation and browser history»). Клиентская навигация рендерит цель в
 * том же JS-дереве и везёт RSC-кэш и react-query heap предыдущего документа
 * в новую страницу — после выхода «Назад» возвращал бы кабинет без сетевого
 * запроса, после входа в кабинет уезжали бы гостевые RSC-остатки. Новый
 * документ сносит heap целиком; replace не оставляет предыдущий документ в
 * history. Bfcache-ближний этого рубежа — корневой BfcacheGuard.
 */
export function hardReplace(href: string): void {
    window.location.replace(href);
}
