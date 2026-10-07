import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import {DEFAULT_PROPERTY_SORT} from '../lib/property-sort';
import {PropertiesSortMenu, PropertiesSortRow} from './PropertiesPage';

/** Плоское дерево React-элементов ряда сортировки без рендера: канон
 * тестов фронта — node без DOM (vitest.config.mts); ряд без хуков можно
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
  // Чип сортировки (PropertiesSortMenu) досматриваем только как элемент:
  // внутрь он уводит в PickerMenu с хуками — вне React-рендера его не
  // вызвать, да и не нужно (факт присутствия/отсутствия чипа — и есть
  // поведение §7).
}

function collect(node: ReactNode): {
  elements: ReactElement<InspectableProps>[];
  strings: string[];
} {
  const elements: ReactElement<InspectableProps>[] = [];
  const strings: string[] = [];
  flatten(node, elements, strings);
  return {elements, strings};
}

describe('PropertiesSortRow — гейт «Архива» по archived_count (#1233, макеты 3229-94647/3235-74057)', () => {
  it('архивные есть, объектов нет: чип скрыт, «Архив» справа (3229-94647)', () => {
    const row = PropertiesSortRow({
      sort: DEFAULT_PROPERTY_SORT,
      onChange: () => {},
      showChip: false,
      showArchive: true,
      onOpenArchive: () => {},
    });
    const {elements, strings} = collect(row);

    // Чипа нет (сортировать нечего), «Архив» на месте.
    expect(elements.some((element) => element.type === PropertiesSortMenu)).toBe(false);
    expect(strings).toContain('Архив');

    // Ряд выровнен вправо — кнопка на обычном месте, а не у левого края.
    expect(row.props.className).toContain('justify-end');
    expect(row.props.className).not.toContain('justify-between');
    expect(elements.length).toBeGreaterThan(0);
  });

  it('на хабе с объектами и архивом чип слева, «Архив» справа', () => {
    const row = PropertiesSortRow({
      sort: DEFAULT_PROPERTY_SORT,
      onChange: () => {},
      showChip: true,
      showArchive: true,
      onOpenArchive: () => {},
    });
    const {elements, strings} = collect(row);

    expect(elements.some((element) => element.type === PropertiesSortMenu)).toBe(true);
    expect(strings).toContain('Архив');
    expect(row.props.className).toContain('justify-between');
  });

  it('архивных нет — кнопки «Архив» нет (правило владельца, #1233)', () => {
    const withChip = PropertiesSortRow({
      sort: DEFAULT_PROPERTY_SORT,
      onChange: () => {},
      showChip: true,
      showArchive: false,
      onOpenArchive: () => {},
    });
    const {elements, strings} = collect(withChip);

    // Хаб с объектами без архива: чип на месте, «Архива» нет.
    expect(elements.some((element) => element.type === PropertiesSortMenu)).toBe(true);
    expect(strings).not.toContain('Архив');

    const empty = PropertiesSortRow({
      sort: DEFAULT_PROPERTY_SORT,
      onChange: () => {},
      showChip: false,
      showArchive: false,
      onOpenArchive: () => {},
    });
    const emptyCollected = collect(empty);

    // Пустой хаб без архива: ряд не рисует ничего — страница его вовсе
    // не рендерит (макет 3235-74057), компонент на всякий случай пуст.
    expect(emptyCollected.strings).not.toContain('Архив');
    expect(emptyCollected.elements.some((element) => element.type === PropertiesSortMenu)).toBe(false);
  });

  it('колбэк страницы передан в кнопку «Архив»', () => {
    const openArchive = (): void => {};
    const row = PropertiesSortRow({
      sort: DEFAULT_PROPERTY_SORT,
      onChange: () => {},
      showChip: false,
      showArchive: true,
      onOpenArchive: openArchive,
    });
    const {elements, strings} = collect(row);

    expect(strings).toContain('Архив');
    const archiveButton = elements.find((element) => element.props.onClick === openArchive);
    expect(archiveButton).toBeDefined();
    expect(collect(archiveButton).strings).toContain('Архив');
  });
});
