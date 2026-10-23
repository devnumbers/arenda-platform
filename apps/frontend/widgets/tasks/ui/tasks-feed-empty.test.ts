import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { Button, EmptyState } from '@/shared/ui/design';
import { TasksFeedEmpty } from './tasks-feed-empty';

/** Канон верха пустой ленты (замечание владельца 23.10): совсем пустой
 * фид — канон EmptyState без переопределений (DESIGN.md §7, анатомия
 * 3235-74057) — обёртка своего верхнего отступа не несёт, pt-14 состояния
 * отсчитывается от строки заголовка хаба, как на «Платежах» и bareEmpty
 * «Объектов» (решение владельца 14.10.2026); бывший mt-6 общей обёртки
 * ленты съезжал иллюстрацию на 24 вниз относительно других хабов
 * (56 вместо канонных pt-14). Живой экран с хуками функцией не
 * вызывается, идиома канон-тестов фронта — ветка пустоты вынесена в
 * чистый компонент; анатомия проверяется по props элементов (без
 * рендера DOM среда vitest — node). */

type InspectableProps = {
  children?: ReactNode;
  className?: string;
};

describe('TasksFeedEmpty (верх пустой ленты)', () => {
  const screen = TasksFeedEmpty({ onCreate: () => undefined }) as ReactElement<InspectableProps>;

  it('обёртка без своего верхнего отступа — pt-14 состояния ведёт от заголовка хаба', () => {
    const className = String(screen.props.className);
    expect(className).not.toMatch(/(^|\s)mt-\S/);
    // Нижний ритм страницы сохранён — как у остальных веток ленты.
    expect(className).toContain('pb-6');
  });

  it('анатомия канона §7: канонный EmptyState, «Задач нет» и CTA Button size small', () => {
    const state = findElement(screen.props.children, (element) => element.type === EmptyState);
    expect(state).not.toBeNull();
    const props = state?.props as {
      title?: string;
      description?: string;
      action?: ReactElement;
    };
    expect(props.title).toBe('Задач нет');
    expect(String(props.description)).toMatch(/^Добавьте задачу/);

    // CTA пустых состояний — Button size="small" radius="m" (44/12, §7).
    const action = props.action;
    expect(action?.type).toBe(Button);
    expect((action?.props as { size?: string; radius?: string }).size).toBe('small');
    expect((action?.props as { size?: string; radius?: string }).radius).toBe('m');
  });
});

function findElement(
  node: ReactNode,
  predicate: (element: ReactElement<InspectableProps>) => boolean,
): ReactElement<InspectableProps> | null {
  if (Array.isArray(node)) {
    for (const child of node) {
      const found = findElement(child, predicate);
      if (found !== null) {
        return found;
      }
    }
    return null;
  }
  if (
    node !== null &&
    typeof node === 'object' &&
    'props' in (node as ReactElement)
  ) {
    const element = node as ReactElement<InspectableProps>;
    if (predicate(element)) {
      return element;
    }
    return findElement(element.props.children, predicate);
  }
  return null;
}
