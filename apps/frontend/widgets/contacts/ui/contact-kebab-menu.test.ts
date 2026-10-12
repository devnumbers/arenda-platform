import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { ContactKebabMenu } from './contact-kebab-menu';

/** Плоское дерево React-элементов без рендера: канон тестов фронта — node
 * без DOM (CODING_STANDARDS «Testing»); меню без хуков вызывается как
 * функция и инспектируется поэлементно (прецедент phone-step.test.ts).
 * Radix-пункты не рендерятся вне открытия — инспекция идёт по props
 * локального MenuItem (label/onSelect), не по строкам дерева. */
type InspectableElement = ReactElement<{ children?: ReactNode; [key: string]: unknown }>;

function flatten(node: ReactNode, into: InspectableElement[]): void {
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, into);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    return;
  }
  const element = node as InspectableElement;
  into.push(element);
  flatten(element.props.children, into);
}

function inspect(props: Parameters<typeof ContactKebabMenu>[0]): InspectableElement[] {
  const elements: InspectableElement[] = [];
  flatten(ContactKebabMenu(props), elements);
  return elements;
}

describe('ContactKebabMenu — кебаб карточки контакта, только удаление (карта #1332)', () => {
  it('пункт «Удалить» в меню несёт обработчик подтверждения', () => {
    const onDelete = (): void => {};
    const elements = inspect({ onDelete });

    const deleteItem = elements.find((el) => el.props.label === 'Удалить');
    expect(deleteItem).toBeDefined();
    expect(deleteItem?.props.onSelect).toBe(onDelete);
  });

  it('пункта «Изменить» нет — правка переехала в тап по карточке «Информация»', () => {
    const elements = inspect({ onDelete: (): void => {} });

    expect(elements.some((el) => el.props.label === 'Изменить')).toBe(false);
  });
});
