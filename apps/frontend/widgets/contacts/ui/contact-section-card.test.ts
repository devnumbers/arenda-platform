import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { SmallArrowRight } from '@/shared/assets/icons';
import { ContactSectionCard } from './contact-section-card';

/** Плоское дерево React-элементов без рендера: канон тестов фронта — node
 * без DOM (CODING_STANDARDS «Testing»); карточка без хуков вызывается как
 * функция и инспектируется поэлементно (прецедент phone-step.test.ts). */
type InspectableElement = ReactElement<{ children?: ReactNode; [key: string]: unknown }>;

function flatten(node: ReactNode, into: InspectableElement[], strings: string[]): void {
  if (typeof node === 'string') {
    strings.push(node);
    return;
  }
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, into, strings);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    return;
  }
  const element = node as InspectableElement;
  into.push(element);
  flatten(element.props.children, into, strings);
}

function inspect(props: Parameters<typeof ContactSectionCard>[0]): {
  elements: InspectableElement[];
  strings: string[];
} {
  const elements: InspectableElement[] = [];
  const strings: string[] = [];
  flatten(ContactSectionCard(props), elements, strings);
  return { elements, strings };
}

describe('ContactSectionCard — секционная карточка (Heading-канон макета 3304:75726)', () => {
  it('заголовок секции в дереве, контент проходит сквозь карточку', () => {
    const { elements, strings } = inspect({
      title: 'Информация',
      children: 'строка значения',
    });

    expect(strings).toContain('Информация');
    expect(strings).toContain('строка значения');
    expect(elements.some((el) => el.props['data-testid'] === 'contact-section-card')).toBe(true);
  });

  it('с href: заголовок — ссылка на адрес секции, вся площадь — скрытая ссылка-оверлей, шеврон декоративен', () => {
    const { elements } = inspect({
      title: 'Объект',
      href: '/properties/prop-1',
      children: null,
    });

    const links = elements.filter((el) => el.props.href === '/properties/prop-1');
    // Заголовок-ссылка (клавиатура/скринридер) + оверлей (вся площадь).
    expect(links).toHaveLength(2);
    // Оверлей скрыт от ассистивных технологий и не в tab-порядке.
    expect(links.some((el) => el.props['aria-hidden'] === true && el.props.tabIndex === -1)).toBe(
      true,
    );
    // Шеврон у заголовка — декоративная иконка канона.
    const chevron = elements.find((el) => el.type === SmallArrowRight);
    expect(chevron).toBeDefined();
    expect(chevron?.props['aria-hidden']).toBe(true);
  });

  it('без href: ссылок нет — секция статична, шеврона нет', () => {
    const { elements } = inspect({ title: 'Заметка', children: null });

    expect(elements.some((el) => el.props.href !== undefined)).toBe(false);
  });
});
