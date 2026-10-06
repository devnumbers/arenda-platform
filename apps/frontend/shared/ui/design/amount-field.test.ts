import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { AmountFieldView } from './amount-field';

/**
 * Дерево элементов дисплея суммы без рендера: AmountField держит буфер
 * в состоянии (хуки), поэтому node-тесты вызывает его stateless-вьюху —
 * канон button.test.ts / wizard-chrome.test.ts (vitest — node без DOM).
 * Контракт тапа (#1151, research #1148, раздел C): вся область дисплея
 * «0 ₽» — цель тапа — инпут расширен за пределы текста измерителя, тап
 * по любому месту дисплея нативный, по самому editable (фокус из жеста —
 * единственный поднимающий клавиатуру путь на iOS, WebKit bug 195884).
 */
type InspectableProps = {
  children?: ReactNode;
  className?: string;
  ref?: unknown;
  disabled?: boolean;
  'aria-label'?: string;
  'aria-hidden'?: boolean;
  inputMode?: string;
  value?: string;
};

function flatten(
  node: ReactNode,
  elements: ReactElement<InspectableProps>[],
): void {
  if (typeof node === 'string') {
    return;
  }
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, elements);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    // числа, false/undefined (условный JSX), прочие листья
    return;
  }
  const element = node as ReactElement<InspectableProps>;
  elements.push(element);
  flatten(element.props.children, elements);
}

function tree(node: ReactNode): ReactElement<InspectableProps>[] {
  const elements: ReactElement<InspectableProps>[] = [];
  flatten(node, elements);
  return elements;
}

const NOOP = (): void => undefined;

const REF = (): void => undefined;

function renderView(disabled?: boolean): ReactElement<InspectableProps> {
  return AmountFieldView({
    buffer: '',
    inputRef: REF,
    onInputChange: NOOP,
    onFocus: NOOP,
    onBlur: NOOP,
    disabled,
    label: 'Сумма',
  });
}

describe('AmountFieldView — вся область дисплея тапабельна (#1151)', () => {
  it('инпут расширен за измеритель по горизонтали: зона тапа накрывает «₽» и воздух вокруг цифр', () => {
    const input = tree(renderView()).find((el) => el.type === 'input');
    expect(String(input?.props.className)).toContain('-inset-x-12');
  });

  it('расширение только по горизонтали: вертикаль (высота строки) не раздвигается', () => {
    const input = tree(renderView()).find((el) => el.type === 'input');
    const className = String(input?.props.className);
    expect(className).toContain('inset-y-0');
    expect(className).not.toMatch(/-inset-y/);
  });

  it('символ рубля — снаружи инпута соседним aria-hidden элементом (канон 2026-08-26)', () => {
    const elements = tree(renderView());
    const inputIndex = elements.findIndex((el) => el.type === 'input');
    const roubleIndex = elements.findIndex((el) => el.props.children === '₽');
    expect(roubleIndex).toBeGreaterThan(-1);
    // «₽» идёт после инпута, за пределами измерителя
    expect(roubleIndex).toBeGreaterThan(inputIndex);
    expect(elements[roubleIndex]?.props['aria-hidden']).toBe(true);
  });

  it('маунт-фокус #1151: ref инпута — колбэк из пропа (фокус в задаче жеста)', () => {
    const input = tree(renderView()).find((el) => el.type === 'input');
    expect(input?.props.ref).toBe(REF);
  });
});

describe('AmountFieldView — канон поля сохранён', () => {
  it('input: имя для скринридеров, цифровая клавиатура ОС, управляемое значение', () => {
    const input = tree(renderView()).find((el) => el.type === 'input');
    expect(input?.props['aria-label']).toBe('Сумма');
    expect(input?.props.inputMode).toBe('decimal');
    expect(input?.props.value).toBe('');
  });

  it('disabled — приглушение на корне и disable инпута', () => {
    const root = renderView(true);
    expect(String(root.props.className)).toContain('opacity-50');
    const input = tree(root).find((el) => el.type === 'input');
    expect(input?.props.disabled).toBe(true);
  });
});
