import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';

// Обсервер пересоздания не дублируется (канон плана 032: «тесты хука не
// дублировать») — хвост тестируется как чистая функция рендера с заглушкой
// хука: vitest-окружение node без DOM, рендер недоступен.
vi.mock('@/shared/lib/hooks/useInfiniteScroll', () => ({
  useInfiniteScroll: () => undefined,
}));

import { Button } from './button';
import { ErrorCard } from './error-card';
import { LoadingMoreIndicator } from './loading-more-indicator';
import { InfiniteQueryTail } from './infinite-query-tail';

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

function makeQuery(overrides: {
  readonly hasNextPage?: boolean | undefined;
  readonly isFetchingNextPage?: boolean;
  readonly isError?: boolean;
  readonly fetchNextPage?: () => Promise<unknown>;
} = {}): Parameters<typeof InfiniteQueryTail>[0]['query'] {
  return {
    hasNextPage: true,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };
}

function render(query: Parameters<typeof InfiniteQueryTail>[0]['query']): {
  elements: ReactElement<InspectableProps>[];
  strings: string[];
  tree: ReactNode;
} {
  const tree = InfiniteQueryTail({ query, tone: 'base' });
  const elements: ReactElement<InspectableProps>[] = [];
  const strings: string[] = [];
  flatten(tree, elements, strings);
  return { elements, strings, tree };
}

/** Инвокированная карточка ошибки из дерева хвоста — с гвардом вместо
 * non-null assertion (lint-канон). */
function renderedCard(elements: ReactElement<InspectableProps>[]): ReturnType<typeof ErrorCard> {
  const cardElement = elements.find((element) => element.type === ErrorCard);
  if (cardElement === undefined) {
    throw new Error('ErrorCard не найден в дереве хвоста');
  }
  return ErrorCard(cardElement.props as never);
}

describe('InfiniteQueryTail: провал порции (план 032 #1442)', () => {
  it('isError при живых страницах — карточка «Не удалось загрузить еще», sentinel-дива нет', () => {
    const { elements, tree } = render(makeQuery({ isError: true }));

    const cardStrings: string[] = [];
    flatten(renderedCard(elements), [], cardStrings);
    expect(cardStrings).toContain('Не удалось загрузить еще');
    // В дереве хоста слов ошибочной карточки нет — только сам элемент канона.
    const treeStrings: string[] = [];
    flatten(tree, [], treeStrings);
    expect(treeStrings).toEqual([]);
    const sentinel = elements.find(
      (element) =>
        element.type === 'div'
        && (element.props as Record<string, unknown>)['aria-hidden'] === true
        && 'ref' in (element.props as Record<string, unknown>),
    );
    expect(sentinel).toBeUndefined();
  });

  it('клик «Повторить» доигрывает следующую страницу (fetchNextPage, не refetch)', () => {
    const fetchNextPage = vi.fn().mockResolvedValue(undefined);
    const { elements } = render(makeQuery({ isError: true, fetchNextPage }));

    const cardElements: ReactElement<InspectableProps>[] = [];
    flatten(renderedCard(elements), cardElements, []);
    const retry = cardElements.find((element) => element.type === Button);
    expect(retry).toBeDefined();
    if (retry === undefined) {
      return;
    }

    (retry.props as { onClick: () => void }).onClick();

    expect(fetchNextPage).toHaveBeenCalledTimes(1);
  });

  it('фаза повтора — глушение машинное: loading от isFetchingNextPage', () => {
    const { elements } = render(
      makeQuery({ isError: true, isFetchingNextPage: true }),
    );

    const cardElements: ReactElement<InspectableProps>[] = [];
    flatten(renderedCard(elements), cardElements, []);
    const retry = cardElements.find((element) => element.type === Button);
    expect(retry).toBeDefined();
    if (retry === undefined) {
      return;
    }
    const invoked = Button(retry.props);

    expect(invoked.props.disabled).toBe(true);
    expect(invoked.props['aria-busy']).toBe(true);
  });

  it('без isError — прежнее поведение: sentinel + индикатор при догрузке', () => {
    const idle = render(makeQuery({}));
    const sentinel = idle.elements.find(
      (element) =>
        element.type === 'div'
        && (element.props as Record<string, unknown>)['aria-hidden'] === true,
    );
    expect(sentinel).toBeDefined();
    expect(idle.elements.find((element) => element.type === LoadingMoreIndicator)).toBeUndefined();
    expect(idle.strings).not.toContain('Не удалось загрузить еще');

    const fetching = render(makeQuery({ isFetchingNextPage: true }));
    expect(
      fetching.elements.find((element) => element.type === LoadingMoreIndicator),
    ).toBeDefined();
  });

  it('hasNextPage false — хвост пуст, ошибка не рисуется', () => {
    const tree = InfiniteQueryTail({
      query: makeQuery({ hasNextPage: false, isError: true }),
      tone: 'base',
    });
    expect(tree).toBeNull();
  });
});
