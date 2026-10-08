import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { PropertyStatusSheet } from './PropertyDetailActions';

/** Плоское дерево React-элементов шита без рендера: канон тестов
 * фронта — node без DOM (vitest.config.mts); шит без хуков можно
 * вызывать как функцию и инспектировать элементы (как в
 * shared/ui/design/button.test.ts). */
type InspectableProps = {
  children?: ReactNode;
  className?: string;
  onClick?: () => void;
};

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
  // Modal/ModalContent досматриваем как элементы: внутрь они уводят
  // в Radix/vaul с хуками — вне React-рендера их не вызвать, да и
  // не нужно (канон Modal §2 DESIGN.md).
}

function collect(node: ReactNode): {
  elements: ReactElement<InspectableProps>[];
  strings: string[];
} {
  const elements: ReactElement<InspectableProps>[] = [];
  const strings: string[] = [];
  flatten(node, elements, strings);
  return { elements, strings };
}

function renderSheet(onClose: () => void): ReturnType<typeof collect> {
  return collect(
    PropertyStatusSheet({
      open: true,
      onOpenChange: (open) => {
        if (!open) {
          onClose();
        }
      },
      items: [{ key: 'start-rental', label: 'Начать аренду' }],
      onAction: () => {},
    }),
  );
}

describe('PropertyStatusSheet — «Отменить» в контейнере пунктов (#1241)', () => {
  it('«Отменить» той же формы, что пункты: контейнер h-14 rounded-button bg-surface-muted', () => {
    let closed = false;
    const { elements, strings } = renderSheet(() => {
      closed = true;
    });

    expect(strings).toContain('Отменить');
    const cancelButton = elements.find(
      (element) =>
        element.props.onClick !== undefined &&
        collect(element).strings.includes('Отменить'),
    );
    expect(cancelButton).toBeDefined();

    // Контейнер как у пунктов меню — решение владельца #1241 (на макете
    // «Отменить» плоская, правится на форму контейнера): высота ряда,
    // скругление кнопок и фон surface-muted, во всю ширину.
    const classes = String(cancelButton?.props.className ?? '');
    expect(classes).toContain('h-14');
    expect(classes).toContain('w-full');
    expect(classes).toContain('rounded-button');
    expect(classes).toContain('bg-surface-muted');
    expect(classes).not.toContain('rounded-pill');

    // Тап «Отменить» только закрывает шит — действие пунктов не триггерит.
    cancelButton?.props.onClick?.();
    expect(closed).toBe(true);
  });

  it('пункты меню — образец формы: тот же контейнер h-14 rounded-button bg-surface-muted', () => {
    const { elements, strings } = renderSheet(() => {});

    expect(strings).toContain('Начать аренду');
    const menuItem = elements.find(
      (element) =>
        element.props.onClick !== undefined &&
        collect(element).strings.includes('Начать аренду'),
    );
    expect(menuItem).toBeDefined();

    const classes = String(menuItem?.props.className ?? '');
    expect(classes).toContain('h-14');
    expect(classes).toContain('rounded-button');
    expect(classes).toContain('bg-surface-muted');
  });
});
