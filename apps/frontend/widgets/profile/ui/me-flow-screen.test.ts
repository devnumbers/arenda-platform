import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { MeFlowSuccessScreen } from './MeFlowScreen';

/** Плоское дерево React-элементов шита успеха без рендера: канон тестов
 * фронта — node без DOM (vitest.config.mts); компонент без хуков
 * вызывается как функция и инспектируется поэлементно (канон —
 * email-step.test.ts). */
type InspectableElement = ReactElement<{ children?: ReactNode; [key: string]: unknown }>;

function flatten(node: ReactNode, strings: string[]): void {
  if (typeof node === 'string') {
    strings.push(node);
    return;
  }
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, strings);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    return;
  }
  const element = node as InspectableElement;
  flatten(element.props.children, strings);
}

function stringsOf(node: ReactNode): string[] {
  const strings: string[] = [];
  flatten(node, strings);
  return strings;
}

const baseProps = {
  message: 'Новая электронная почта new@example.com',
  onClose: () => {},
} as const;

describe('MeFlowSuccessScreen — шит успеха потоков смены', () => {
  it('сообщение — заголовком, «Хорошо» — кнопкой шита; без сабтайтла других строк нет', () => {
    const strings = stringsOf(MeFlowSuccessScreen(baseProps));

    // Прежний контракт (почта 105223, телефон 1869-68303): одна строка
    // сообщения и кнопка шита — никаких подписей под заголовком.
    expect(strings).toEqual(['Новая электронная почта new@example.com', 'Хорошо']);
  });

  it('с сабтайтлом подпись идёт под сообщением (#1531: вход теперь по новому адресу)', () => {
    const strings = stringsOf(
      MeFlowSuccessScreen({ ...baseProps, subtitle: 'Вход в аккаунт теперь по этому адресу' }),
    );

    expect(strings).toContain('Новая электронная почта new@example.com');
    expect(strings).toContain('Вход в аккаунт теперь по этому адресу');
  });
});
