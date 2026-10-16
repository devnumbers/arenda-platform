/**
 * Реестр «свежих» строк живой ленты «Истории» (тикет #880, механика F
 * демо #879): строки, влитые live-догоном (#718), получают анимацию
 * появления (row-in 350мс) и метку «новое», пока читатель их не увидит —
 * доскролл до низа или канонная пауза у читателя на дне. Модульный
 * синглтон вкладки рядом с lastLiveMergeTimes (hooks.ts): отметка — в
 * момент live-влития в кэш, чтение — на экране дельтой от снимка краёв,
 * гашение — когда читатель доехал до свежих строк.
 */

/** scopeKey → id строки → момент влития (performance.now()-подобные мс). */
const freshMerges = new Map<string, Map<string, number>>();

/** Отмечает строки, влитые live-догоном в кэш; повторное влитие той же
 * строки (гонка с onOpen-перечитыванием) обновляет метку. */
export function noteFreshFeedEntryIds(scopeKey: string, ids: readonly string[], at: number): void {
  if (ids.length === 0) {
    return;
  }
  let byId = freshMerges.get(scopeKey);
  if (byId === undefined) {
    byId = new Map<string, number>();
    freshMerges.set(scopeKey, byId);
  }
  for (const id of ids) {
    byId.set(id, at);
  }
}

/** Строки, влитые ПОЗЖЕ момента at — дельта экрана от снимка краёв. */
export function freshFeedEntryIdsMergedSince(scopeKey: string, at: number): string[] {
  const byId = freshMerges.get(scopeKey);
  if (byId === undefined) {
    return [];
  }
  const fresh: string[] = [];
  for (const [id, mergedAt] of byId) {
    if (mergedAt > at) {
      fresh.push(id);
    }
  }
  return fresh;
}

/** Читатель увидел свежие строки (доскролл до низа или канонная пауза на
 * дне) — метки гаснут. */
export function acknowledgeFreshFeedEntryIds(scopeKey: string): void {
  freshMerges.delete(scopeKey);
}
