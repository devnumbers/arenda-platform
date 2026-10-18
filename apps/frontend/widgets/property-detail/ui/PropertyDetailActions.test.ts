import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { Cancel } from '@/shared/assets/icons';
import { propertyActionIcon, PropertyStatusSheet } from './PropertyDetailActions';

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
      items: [{ key: 'start-rental', label: 'Начать аренду', danger: false }],
      onAction: () => {},
    }),
  );
}

describe('PropertyStatusSheet — «Отменить» в контейнере пунктов (#1241)', () => {
  it('«Отменить» — контейнер формы пунктов, но белый (макет 1554-100758), отклик по канону secondary', () => {
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

    // Контейнер как у пунктов, но белый (решение владельца #1241: макеты
    // 1554-100758/1581-53679 — фон «Отменить» var(--white) при серых
    // пунктах; в покое контейнер читается геометрией, не цветом).
    const classList = String(cancelButton?.props.className ?? '').split(' ');
    expect(classList).toContain('h-14');
    expect(classList).toContain('w-full');
    expect(classList).toContain('rounded-button');
    expect(classList).toContain('bg-surface');
    // Отклик — по канону Button secondary (решение владельца 08.10).
    expect(classList).toContain('hover:bg-surface-muted-hover');
    expect(classList).toContain('active:bg-surface-muted-active');
    // Не серый пункт и не прежняя плоская пилюля.
    expect(classList).not.toContain('bg-surface-muted');
    expect(classList).not.toContain('rounded-pill');

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

describe('«Удалить аренду» — глиф и цвет по макету 3460-78651 (решение владельца 17.10)', () => {
  it('иконка строки — канонный Cancel (✕), не TrashBin как у «Удалить объект»', () => {
    const icon = propertyActionIcon('delete-rental');
    expect(icon).not.toBeNull();
    expect(icon?.type).toBe(Cancel);
  });

  it('danger-пункт шита статуса несёт красную подпись и красный глиф, обычные — content', () => {
    const { elements } = collect(
      PropertyStatusSheet({
        open: true,
        onOpenChange: () => {},
        items: [
          { key: 'delete-rental', label: 'Удалить аренду', danger: true },
          { key: 'start-maintenance', label: 'Объект на ремонте', danger: false },
        ],
        onAction: () => {},
      }),
    );

    // Красный глиф: text-danger на самой кнопке пункта (иконка красится
    // currentColor), подпись — своим спаном.
    const dangerRow = elements.find(
      (element) =>
        String(element.props.className ?? '').includes('bg-surface-muted') &&
        collect(element).strings.includes('Удалить аренду'),
    );
    expect(String(dangerRow?.props.className)).toContain('text-danger');

    const labelClassOf = (label: string): string => {
      // Прямой ребёнок — строка-лейбл: иначе первым находится контейнер
      // меню, в поддереве которого лейблы тоже лежат.
      const span = elements.find((element) => element.props.children === label);
      return String(span?.props.className ?? '');
    };
    expect(labelClassOf('Удалить аренду')).toContain('text-danger');
    expect(labelClassOf('Объект на ремонте')).toContain('text-content');
    expect(labelClassOf('Объект на ремонте')).not.toContain('text-danger');
  });
});
