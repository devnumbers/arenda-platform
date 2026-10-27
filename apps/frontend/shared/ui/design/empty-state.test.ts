import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { EmptyState } from './empty-state';

/** Плоское дерево React-элементов без рендера: канон тестов фронта —
 * node без DOM (vitest.config.mts), тот же стиль, что error-card.test.ts —
 * EmptyState без хуков вызывается как функция и инспектируется. */
type InspectableProps = { children?: ReactNode; className?: string };

function flatten(
  node: ReactNode,
  elements: ReactElement<InspectableProps>[],
): void {
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, elements);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    return;
  }
  const element = node as ReactElement<InspectableProps>;
  elements.push(element);
  flatten(element.props.children, elements);
}

/** Заголовок — единственный h2 в дереве пустого состояния. */
function titleOf(tree: ReactNode): ReactElement<InspectableProps> {
  const elements: ReactElement<InspectableProps>[] = [];
  flatten(tree, elements);
  const heading = elements.find((element) => element.type === 'h2');
  if (!heading) {
    throw new Error('h2 не найден в дереве EmptyState');
  }
  return heading;
}

describe('EmptyState: класс заголовка', () => {
  it('без titleClassName — канон 24/28 semibold', () => {
    const tree = EmptyState({ imageSrc: '/images/empty.webp', title: 'Пусто' });

    expect(titleOf(tree).props.className).toBe(
      'text-[24px] font-semibold leading-7 text-content',
    );
  });

  it('titleClassName перекрывает размер/вес, цвет канона остаётся (service-титул уведомлений, макеты 3178-86086/3187-88176)', () => {
    const tree = EmptyState({
      imageSrc: '/images/notifications/empty-bell.webp',
      title: 'Нет уведомлений',
      titleClassName: 'text-base leading-[18px] font-normal',
    });

    const className = titleOf(tree).props.className ?? '';
    expect(className).toContain('text-base');
    expect(className).toContain('leading-[18px]');
    expect(className).toContain('font-normal');
    expect(className).toContain('text-content');
    expect(className).not.toContain('text-[24px]');
    expect(className).not.toContain('font-semibold');
    expect(className).not.toContain('leading-7');
  });
});
