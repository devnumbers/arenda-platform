import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { Button } from './button';
import { ErrorCard } from './error-card';

/** Плоское дерево React-элементов без рендера: канон тестов фронта —
 * node без DOM (vitest.config.mts), тот же стиль, что button.test.ts —
 * ErrorCard без хуков вызывается как функция и инспектируется. */
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
    return;
  }
  const element = node as ReactElement<InspectableProps>;
  elements.push(element);
  flatten(element.props.children, elements, strings);
}

/** Кнопка «Повторить» — единственный Button в дереве карточки; глушение
 * вычисляется внутри Button, поэтому элемент доинвокается (канон
 * button.test.ts — компонент зовётся как функция). */
function retryButton(card: ReturnType<typeof ErrorCard>): ReactElement<Record<string, unknown>> {
  const elements: ReactElement<InspectableProps>[] = [];
  const strings: string[] = [];
  flatten(card, elements, strings);
  const buttonElement = elements.find((element) => element.type === Button);
  if (!buttonElement) {
    throw new Error('Button «Повторить» не найден в дереве ErrorCard');
  }
  expect(strings).toContain('Повторить');
  return Button(buttonElement.props) as ReactElement<Record<string, unknown>>;
}

describe('ErrorCard: фаза повтора (план 031 #1442, граница #1114 из решения #1420)', () => {
  it('при loading кнопка глушится машинно: disabled, aria-busy, data-loading, текст видим', () => {
    const card = ErrorCard({ title: 'Не удалось загрузить контакты', onRetry: () => {}, loading: true });
    const button = retryButton(card);

    expect(button.props.disabled).toBe(true);
    expect(button.props['aria-busy']).toBe(true);
    expect(button.props['data-loading']).toBe('true');
    const strings: string[] = [];
    flatten(card, [], strings);
    expect(strings).toContain('Повторить');
  });

  it('без loading кнопка активна', () => {
    const card = ErrorCard({ title: 'Ошибка', onRetry: () => {} });
    const button = retryButton(card);

    expect(button.props.disabled).toBeFalsy();
    expect(button.props['aria-busy']).toBeFalsy();
  });

  it('клик по кнопке вызывает onRetry', () => {
    const onRetry = vi.fn();
    const card = ErrorCard({ title: 'Ошибка', onRetry });
    const button = retryButton(card);

    (button.props.onClick as () => void)();

    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
