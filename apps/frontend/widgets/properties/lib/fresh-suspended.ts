/** Семантика снапшота подвесших (#880) — чистое ядро layout-эффекта хаба
 * «Объекты» (PropertiesPage): сверяет id подвесших прошлого рендера с
 * доставкой meta-запроса /properties и решает, какие карточки приехали
 * живым перечитыванием (fresh — blur-in), а какой снапшот передать дальше.
 */

export type FreshSuspendedResolution = {
  /** Снапшот для следующего эффекта: null = доставка ещё не случилась. */
  readonly next: ReadonlySet<string> | null;
  /** Id, появившиеся живым перечитыванием (дельта prev→ids) — blur-in. */
  readonly fresh: ReadonlySet<string>;
};

/** prev — снапшот id прошлого рендера (null = холодный вход: первая
 * доставка без анимации); data — доставка экрана, undefined = фаза
 * загрузки. Пока доставки нет, снапшот ни сеется пустым, ни тратится:
 * пустая загрузка не становится «прошлым рендером», иначе первая
 * настоящая доставка дала бы fresh = все подвесшие разом. */
export function resolveFreshSuspendedIds(
  prev: ReadonlySet<string> | null,
  data: ReadonlyArray<{ readonly propertyId: string }> | undefined,
): FreshSuspendedResolution {
  if (data === undefined) {
    return { next: prev, fresh: new Set() };
  }
  const next = new Set(data.map((placeholder) => placeholder.propertyId));
  if (prev === null) {
    return { next, fresh: new Set() };
  }
  const fresh = new Set([...next].filter((id) => !prev.has(id)));
  return { next, fresh };
}
