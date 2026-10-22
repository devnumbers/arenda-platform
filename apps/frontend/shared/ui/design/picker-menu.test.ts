import { describe, expect, it } from 'vitest';
import { MenuItem } from './menu';
import {
  pickerMenuContentPinClasses,
  pickerMenuPinnedRowClasses,
} from './picker-menu';

/** Пин прежнего спека пикера (макет 1603-94487): новый кебаб-спек
 * 3183-86541 сортировки не трогает (решение владельца 22.10). Контракт:
 * пин-константы побеждают новый базовый спек пункта через cn, а ряд
 * собирается тем же MenuItem — слияние проверяется на настоящем cn. */

describe('Пин пикера — макет 1603-94487 вместо нового кебаб-спека', () => {
  it('ряды: высота 44 (h-11 + min-h-11 поверх min-h-12) и px 20 поверх px-4', () => {
    const merged = String(
      MenuItem({
        className: pickerMenuPinnedRowClasses,
        children: 'По дате создания',
      }).props.className,
    );
    expect(merged).toContain('h-11');
    expect(merged).toContain('min-h-11');
    expect(merged).toContain('px-5');
    expect(merged).not.toContain('min-h-12');
    expect(merged).not.toContain('px-4');
    // Межстрочный 16px нового пункта совпадает с макетом пикера — не пиним.
    expect(merged).toContain('leading-4');
  });

  it('карточка: py 12px и тень 0 8px 24px — прежние, поверх p-0.5 и тени-12 базы', () => {
    expect(pickerMenuContentPinClasses).toContain('py-3');
    expect(pickerMenuContentPinClasses).toContain(
      'shadow-[0_8px_24px_rgba(23,26,28,0.12)]',
    );
  });
});
