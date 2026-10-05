import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { Button } from './button';

/** Плоское дерево React-элементов кнопки без рендера: канон тестов
 * фронта — node без DOM (vitest.config.mts); Button без хуков можно
 * вызывать как функцию и инспектировать элементы (исключение канона
 * § Testing зафиксировано в CODING_STANDARDS.md 30.09). */
type InspectableProps = { children?: ReactNode; className?: string };

function flatten(
  node: ReactNode,
  elements: ReactElement<InspectableProps>[],
  strings: string[],
): void {
  if (typeof node === 'string') {
    strings.push(node);
    return;
  }
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, elements, strings);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    // числа, false/undefined (условный JSX), прочие листья
    return;
  }
  const element = node as ReactElement<InspectableProps>;
  elements.push(element);
  flatten(element.props.children, elements, strings);
}

describe('Button при loading (решение владельца 30.09: без иконки загрузки)', () => {
  it('кнопка дизейблится и помечается data-loading/aria-busy', () => {
    const button = Button({ children: 'Сохранить', loading: true });
    expect(button.props.disabled).toBe(true);
    expect(button.props['data-loading']).toBe('true');
    expect(button.props['aria-busy']).toBe(true);
  });

  it('контент с иконками остаётся в дереве — спиннера нет', () => {
    const button = Button({
      children: 'Сохранить',
      leadingIcon: 'i',
      trailingIcon: 't',
      loading: true,
    });
    const elements: ReactElement<InspectableProps>[] = [];
    const strings: string[] = [];
    flatten(button, elements, strings);
    const classes = elements
      .map((element) => element.props.className ?? '')
      .join(' ');
    expect(classes).not.toContain('animate-spin');
    expect(strings).toEqual(expect.arrayContaining(['i', 'Сохранить', 't']));
  });

  it('без loading кнопка остаётся кликабельной — disabled не включается', () => {
    const button = Button({ children: 'Сохранить' });
    expect(button.props.disabled).toBeFalsy();
    expect(button.props['data-loading']).toBeUndefined();
  });

  it('инвариант #1119: loading глушит и при явном disabled=false (disabled || loading)', () => {
    const button = Button({ children: 'Сохранить', loading: true, disabled: false });
    expect(button.props.disabled).toBe(true);
  });
});
