import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { Edit, TrashBin } from '@/shared/assets/icons';
import { IconButton, MenuContent, MenuItem } from '@/shared/ui/design';
import { ContactKebabMenu } from './contact-kebab-menu';

/** Плоское дерево React-элементов без рендера: канон тестов фронта — node
 * без DOM (CODING_STANDARDS «Testing»); Radix-пункты не рендерятся вне
 * открытия — инспекция идёт по props канонического MenuItem дизайн-слоя
 * (children — текст пункта, onSelect, icon). */
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

function menuItems(elements: InspectableElement[]): InspectableElement[] {
  return elements.filter((el) => el.type === MenuItem);
}

/** Пункт по индексу с guard-ошибкой: noUncheckedIndexedAccess не даёт
 * читать элементы массива без сужения (strict tsconfig). */
function itemAt(items: InspectableElement[], index: number): InspectableElement {
  const item = items.at(index);
  if (item === undefined) {
    throw new Error(`пункт меню ${index} не найден`);
  }
  return item;
}

describe('ContactKebabMenu — кебаб карточки контакта: правка и удаление', () => {
  const onEdit = (): void => {};
  const onDelete = (): void => {};

  it('меню — канон дизайн-слоя (MenuContent), триггер — прежний кебаб IconButton', () => {
    const elements = inspect({ onEdit, onDelete });

    const content = elements.find((el) => el.type === MenuContent);
    expect(content).toBeDefined();
    expect(content?.props.collisionPadding).toBe(24);
    const trigger = elements.find((el) => el.type === IconButton);
    expect(trigger?.props.label).toBe('Меню контакта');
  });

  it('первый пункт «Изменить контакт» — иконка Edit, обработчик правки', () => {
    const items = menuItems(inspect({ onEdit, onDelete }));

    const edit = itemAt(items, 0);
    expect(edit.props.children).toBe('Изменить контакт');
    expect(edit.props.onSelect).toBe(onEdit);
    const icon = edit.props.icon as InspectableElement;
    expect(icon.type).toBe(Edit);
    expect(String(icon.props.className)).toBe('h-6 w-6');
  });

  it('«Удалить» последний — иконка TrashBin и danger-стиль канона (text-error)', () => {
    const items = menuItems(inspect({ onEdit, onDelete }));

    expect(items).toHaveLength(2);
    const del = itemAt(items, 1);
    expect(del.props.children).toBe('Удалить');
    expect(del.props.onSelect).toBe(onDelete);
    expect(String(del.props.className)).toContain('text-error');
    const icon = del.props.icon as InspectableElement;
    expect(icon.type).toBe(TrashBin);
    expect(String(icon.props.className)).toContain('text-error');
  });
});
