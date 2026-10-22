import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Контракт инлайновой единицы измерения поля (решение владельца
 * 2026-10-22, макеты 1218:54304 — характеристики объекта, 1270:46905 —
 * арендная плата; референс поведения — инпут цены Яндекс.Аренды): единица
 * — часть строки значения, серый декоративный текст, который следует за
 * введённым текстом; у пустого поля стоит у левого края. Прежние каноны
 * сняты: префикс-единица слева до ввода и правый статичный суффикс.
 * Механика без JS-замера: невидимый двойник текста занимает ширину
 * значения в потоке, инпут лежит поверх абсолютом.
 */

const textFieldSource = readFileSync(
  fileURLToPath(new URL('./text-field.tsx', import.meta.url)),
  'utf8',
);

describe('Инлайновая единица TextField — проп и разметка', () => {
  it('проп inlineSuffix задокументирован макетами, prefix/suffix сняты', () => {
    expect(textFieldSource).toContain('readonly inlineSuffix?: ReactNode;');
    expect(textFieldSource).toContain('1270:46905');
    expect(textFieldSource, 'префикс-единица слева снята').not.toContain(
      'readonly prefix?: ReactNode;',
    );
    expect(textFieldSource, 'правый статичный суффикс снят').not.toContain(
      'readonly suffix?: ReactNode;',
    );
  });

  it('двойник текста: невидимый, в потоке, без переносов, тот же value', () => {
    expect(textFieldSource).toContain(
      'className="invisible whitespace-pre text-base leading-[18px]"',
    );
    const ghost = textFieldSource.indexOf('invisible whitespace-pre');
    const branch = textFieldSource.slice(ghost, ghost + 800);
    expect(branch, 'двойник рендерит unitValue').toContain('{unitValue}');
  });

  it('инпут лежит поверх двойника абсолютом (каретка и ввод обычные)', () => {
    expect(textFieldSource).toContain('absolute inset-0 px-0');
  });

  it('обёртка держит минимум под каретку и не даёт боксу распирать', () => {
    expect(textFieldSource).toContain(
      'relative inline-flex h-full min-w-[2px] max-w-full items-center overflow-hidden',
    );
  });

  it('цвет единицы живёт на фокусе/значении, не статичный серый', () => {
    expect(textFieldSource).toContain(
      "unitTinted ? 'text-content' : 'text-content-secondary'",
    );
  });

  it('тёмная единица: фокус или непустое значение (аменда владельца 22.10)', () => {
    expect(textFieldSource).toContain('const unitTinted = unitFocused || unitValue.length > 0;');
    expect(textFieldSource).toContain('setUnitFocused(true)');
    expect(textFieldSource).toContain('setUnitFocused(false)');
  });

  it('обработчики фокуса составлены с caller-обработчиками, не заменяют их', () => {
    expect(textFieldSource).toContain('callerFocus?.(event)');
    expect(textFieldSource).toContain('callerBlur?.(event)');
  });

  it('кнопка очистки прижата к правому краю (ml-auto)', () => {
    expect(textFieldSource).toContain('ml-auto');
  });

  it('mousedown по пустой части бокса фокусирует инпут (не в disabled)', () => {
    expect(textFieldSource).toContain(
      'onMouseDown={unitActive && !disabled ? focusInlineUnit : undefined}',
    );
    // Инпут ищется запросом по боксу — caller-ref инпута не трогается;
    // клик по самому инпуту не перехватывается (он ставит каретку кликом),
    // остальное — preventDefault (mousedown по умолчанию унёс бы фокус).
    expect(textFieldSource).toContain("querySelector<HTMLInputElement>('input')");
    expect(textFieldSource).toContain('event.target === input');
    expect(textFieldSource).toContain('event.preventDefault()');
  });

  it('лейбл очистки контекстный — с заголовком поля', () => {
    expect(textFieldSource).toContain('Очистить «${title}»');
  });
});
