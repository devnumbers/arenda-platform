'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';

/**
 * Запись группы параметров в адрес (pre-merge #785, скоуп — пять экранов
 * сортировок): `write(patch, { own })` пишет patch ПОВЕРХ текущих параметров
 * адреса, предварительно сняв все собственные ключи `own`. Поэтому ключ
 * группы, отсутствующий в patch (дефолт), удаляется из адреса, а не остаётся
 * протухшим — урок фикса 0c64fcc3: гард «параметр пуст» вместо снятия
 * оставлял в адресе старое значение, и после перезагрузки возвращалась
 * не та сортировка. Чужие параметры (например, фильтр ленты задач #524)
 * не трогаются. Запись — router.replace без скролла: записей истории не
 * создаёт (конвенция страницы «Объекты», #785). «Дефолтные значения не
 * пишутся» решает serialize-функция фичи: patch несёт только не-дефолты.
 */

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
  readonly write: (
    patch: Record<string, string>,
    options?: { readonly own?: ReadonlyArray<string> },
  ) => void;
} {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  // Ручная мемоизация не нужна: значение не утекает из React (CODING_STANDARDS).
  const write = (
    patch: Record<string, string>,
    options?: { readonly own?: ReadonlyArray<string> },
  ): void => {
    const params = applyUrlParamsPatch(searchParams, patch, options?.own ?? []);
    router.replace(buildUrlWithParams(pathname, params), { scroll: false });
  };

  return { write };
}
