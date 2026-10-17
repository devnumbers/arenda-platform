/** Ручной якорь приписки ленты бесконечного календаря (план 051).

 * Фон: позиция прокрутки при prepend (дорисовка месяцев СВЕРХУ) держалась
 * на нативном scroll anchoring браузера (правка 042). Нативный якорь есть
 * в Chromium (56+) и Firefox (66+), а в WebKit появился только в Safari 27
 * (осень 2026; MDN browser-compat-data): на телефоне владельца (iOS ≤ 26 —
 * и Safari, и Chrome там WebKit) каждая приписка сдвигала весь контент
 * вниз на высоту порции без всякой компенсации — «жёстко лагает» в
 * моментуме; на ПК Chromium компенсировал нативно, поэтому «на ПК нет».
 * Тот же приём уже починил колесо (план 049): позицию после приписки
 * ставим своим кодом до пейнта (useLayoutEffect), одним кадром с DOM-
 * вставкой — сдвиг не закрашивается, моментум между скролл-событием и
 * коммитом сохраняется (пишем в текущий scrollTop). На ленте стоит
 * [overflow-anchor:none] — компенсация ровно одна (наша) на всех движках,
 * без удвоения там, где нативный якорь есть. */

/** Захват перед припиской: ключ и offsetTop текущего первого месяца. */
export type PrependAnchorCapture = {
  readonly key: string;
  readonly top: number;
};

export type PrependRestoreInput = {
  /** offsetTop того же элемента после коммита приписки; null — элемент
   * сошёл со сцены (писать нечего). */
  readonly anchorTop: number | null;
  /** offsetTop элемента до приписки (захват в prepend-ветке handleScroll). */
  readonly capturedTop: number;
};

/** Сколько пикселей приписка добавила над захваченным якорем. Дельта
 * offsetTop одного и того же элемента точна при любых оценках высот
 * вокруг: сверху над ним выросла только приписка. null — восстанавливать
 * нечего (элемент пропал или роста не было — например, захват дожил до
 * чужого коммита без приписки). */
export function planPrependRestore(input: PrependRestoreInput): number | null {
  if (input.anchorTop === null) {
    return null;
  }
  const inserted = input.anchorTop - input.capturedTop;
  return inserted > 0 ? inserted : null;
}
