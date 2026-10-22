import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Контракт MoneyField аренды (решение владельца 2026-10-22, макет
 * 1270:46905): символ «₽» вернулся инлайновым серым суффиксом, следующим
 * за суммой (у пустого поля — у левого края); решение 2026-09-05 «без
 * символа рубля» снято. Компонент унифицирован на канонный TextField —
 * собственный бокс-дубликат снесён; маска суммы и синхронизация DOM
 * сохранены.
 */

const source = readFileSync(
  fileURLToPath(new URL('./wizard-chrome.tsx', import.meta.url)),
  'utf8',
);

const anchor = source.indexOf('export function MoneyField');
const tail = anchor === -1 ? undefined : source.slice(anchor);
const nextExport = tail?.indexOf('\nexport ', 1);
const moneyFieldBody =
  tail !== undefined && nextExport !== undefined && nextExport !== -1
    ? tail.slice(0, nextExport)
    : tail;

describe('MoneyField — инлайновый «₽» на канонном TextField', () => {
  it('MoneyField найден', () => {
    expect(moneyFieldBody).toBeDefined();
  });

  it('единица «₽» — инлайновый суффикс', () => {
    expect(moneyFieldBody).toContain('inlineSuffix="₽"');
  });

  it('маска суммы и синхронизация DOM сохранены', () => {
    expect(moneyFieldBody).toContain('value={groupedAmount(raw)}');
    expect(moneyFieldBody).toContain('sanitizeAmountInput');
    expect(moneyFieldBody).toContain('syncAmountInputDom');
  });

  it('доступное имя поля сохранено (aria-label)', () => {
    expect(moneyFieldBody).toContain('aria-label={ariaLabel}');
  });

  it('собственный бокс-дубликат снесён (рендер через TextField)', () => {
    expect(moneyFieldBody, 'TextField использован').toContain('<TextField');
    expect(moneyFieldBody, 'пл-[18px] в теле нет').not.toContain('pl-[18px]');
    expect(moneyFieldBody, 'Cancel-иконка в теле не рисуется').not.toContain(
      '<Cancel',
    );
  });
});
