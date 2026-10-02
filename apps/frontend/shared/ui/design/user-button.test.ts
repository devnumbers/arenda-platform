import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import Link from 'next/link';
import { UserButton } from './user-button';
import { Skeleton } from './skeleton';

/** Плоское дерево React-элементов ссылки профиля без рендера: канон
 * тестов фронта — node без DOM (vitest.config.mts); UserButton без хуков
 * можно вызывать как функцию и инспектировать элементы (как в
 * button.test.ts). */
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

describe('UserButton — ссылка на профиль (решение владельца 02.10, макет 2329-148674)', () => {
  it('рендерится ссылкой next/link на /profile', () => {
    const link = UserButton({ name: 'Денис' });
    expect(link.type).toBe(Link);
    expect(link.props.href).toBe('/profile');
  });

  it('имя видно на всех ярусах — скрывающих классов в дереве нет', () => {
    const link = UserButton({ name: 'Денис' });
    const elements: ReactElement<InspectableProps>[] = [];
    const strings: string[] = [];
    flatten(link, elements, strings);
    const classes = elements
      .map((element) => element.props.className ?? '')
      .join(' ');
    expect(classes).not.toContain('hidden');
    expect(classes).not.toContain('desktop:inline');
    expect(strings).toContain('Денис');
  });

  it('видимое имя не дублирует доступное — aria-label не ставится', () => {
    const link = UserButton({ name: 'Денис' });
    expect(link.props['aria-label']).toBeUndefined();
  });

  it('пока имени не видно (pending) — скелетон и доступное имя «Профиль: Денис»', () => {
    const link = UserButton({ name: 'Денис', pending: true });
    expect(link.props['aria-label']).toBe('Профиль: Денис');
    const elements: ReactElement<InspectableProps>[] = [];
    const strings: string[] = [];
    flatten(link, elements, strings);
    expect(strings).not.toContain('Денис');
    expect(elements.some((element) => element.type === Skeleton)).toBe(true);
  });

  it('без имени доступное имя — «Профиль»', () => {
    const link = UserButton({});
    expect(link.props['aria-label']).toBe('Профиль');
  });
});
