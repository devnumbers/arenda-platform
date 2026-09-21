'use client';

import {
  usePathname,
  useRouter,
  useSearchParams,
  type ReadonlyURLSearchParams,
} from 'next/navigation';

/**
 * Запись группы параметров в адрес (pre-merge #785; хвосты — #786):
 * `write(patch, { own })` пишет patch ПОВЕРХ текущих параметров адреса,
 * предварительно сняв все собственные ключи `own`. Поэтому ключ группы,
 * отсутствующий в patch (дефолт), удаляется из адреса, а не остаётся
 * протухшим — урок фикса 0c64fcc3: гард «параметр пуст» вместо снятия
 * оставлял в адресе старое значение, и после перезагрузки возвращалась
 * не та сортировка. Чужие параметры (например, фильтр ленты задач #524)
 * не трогаются. Режим записи: `replace` (дефолт) — записей истории не
 * создаёт, конвенция сортировок (страница «Объекты»); `push` — для
 * писателей, которым нужна история («назад» снимает фильтр: #524,
 * #477, #541). «Дефолтные значения не пишутся» решает serialize-функция
 * фичи: patch несёт только не-дефолты. `params` — те же текущие параметры
 * для чтения: хуки-читатели (фильтры #524/#477/#541) берут адрес отсюда
 * же, откуда пишут.
 */

/** Режим записи адреса: replace — без записи истории (сортировки), push —
 * с записью (фильтры, у которых «назад» возвращает без фильтра). */
export type UrlParamsWriteMode = 'replace' | 'push';

export type UseUrlParamsWriteOptions = {
  readonly own?: ReadonlyArray<string>;
  readonly mode?: UrlParamsWriteMode;
};

/**
 * Чистое ядро: патч полного желаемого состояния группы `own` поверх
 * текущих параметров. Поведенческие гарантии — в use-url-params.test.ts.
 */
export function applyUrlParamsPatch(
  current: URLSearchParams,
  patch: Record<string, string>,
  own: ReadonlyArray<string>,
): URLSearchParams {
  const next = new URLSearchParams(current);
  for (const name of own) {
    next.delete(name);
  }
  for (const [name, value] of Object.entries(patch)) {
    next.set(name, value);
  }
  return next;
}

/** Адрес с query или без (пустые параметры — голый pathname). */
export function buildUrlWithParams(pathname: string, params: URLSearchParams): string {
  const query = params.toString();
  return query.length > 0 ? `${pathname}?${query}` : pathname;
}

export function useUrlParams(): {
  /** Только чтение: set/delete адресом не делаются — запись через write. */
  readonly params: ReadonlyURLSearchParams;
  readonly write: (
    patch: Record<string, string>,
    options?: UseUrlParamsWriteOptions,
  ) => void;
} {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  // Ручная мемоизация write не нужна: значение не утекает из React
  // (CODING_STANDARDS), мемоизирует компилятор.
  const write = (
    patch: Record<string, string>,
    options?: UseUrlParamsWriteOptions,
  ): void => {
    const params = applyUrlParamsPatch(searchParams, patch, options?.own ?? []);
    const url = buildUrlWithParams(pathname, params);
    if (options?.mode === 'push') {
      router.push(url, { scroll: false });
    } else {
      router.replace(url, { scroll: false });
    }
  };

  return { params: searchParams, write };
}
