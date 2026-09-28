import type { HistorySegment } from './types';

/**
 * Голая подпись даты в хвосте строки (аудит #876, решение владельца:
 * «только подписи про даты убери»): « (срок DD.MM.YYYY)» операций и задач,
 * « (DD.MM.YYYY – DD.MM.YYYY)» и « (DD.MM.YYYY)» (бессрочная) аренды.
 * Словесные даты («(выселение с 02.10.2025)») и «(Вы)» — не подписи,
 * не срезаются. Бэк (#876, гигиена) такие сегменты больше не кладёт;
 * стрип на фронте чистит и уже записанные строки журнала.
 */
const BARE_DATE_TAIL = /^ \((?:срок \d{2}\.\d{2}\.\d{4}|\d{2}\.\d{2}\.\d{4}(?: – \d{2}\.\d{2}\.\d{4})?)\)$/;

/** Срезает завершающие текстовые сегменты с голой датой (без ссылки). */
export function stripBareDateTails(segments: readonly HistorySegment[]): HistorySegment[] {
  const out = [...segments];
  for (;;) {
    const last = out.at(-1);
    if (last === undefined || last.link !== undefined || !BARE_DATE_TAIL.test(last.text)) {
      break;
    }
    out.pop();
  }
  return out;
}
