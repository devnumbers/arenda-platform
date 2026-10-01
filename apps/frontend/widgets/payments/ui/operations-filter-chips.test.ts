import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { OperationsPeriodChip } from './operations-filter-chips';

/** Чип периода без рендера: канон тестов фронта — node без DOM, компонент
 * без хуков вызывается как функция и инспектируется (идиома button.test.ts). */

type InspectableProps = {
  children?: ReactNode;
  className?: string;
  onClick?: () => void;
  type?: string;
  'aria-hidden'?: boolean;
};

function chipElement(
  props: Parameters<typeof OperationsPeriodChip>[0],
): ReactElement<InspectableProps> {
  return OperationsPeriodChip(props) as ReactElement<InspectableProps>;
}

describe('OperationsPeriodChip (дуальный режим, решение владельца 01.10)', () => {
  it('без onOpen — дисплейный span aria-hidden (скелетоны)', () => {
    const chip = chipElement({ period: null });
    expect(chip.type).toBe('span');
    expect(chip.props['aria-hidden']).toBe(true);
    expect(chip.props.onClick).toBeUndefined();
    expect(chip.props.children).toBe('Период');
  });

  it('с onOpen — кнопка открытия пикера: type=button, onClick, cursor-pointer', () => {
    const onOpen = (): void => {};
    const chip = chipElement({ period: null, onOpen });
    expect(chip.type).toBe('button');
    expect(chip.props.type).toBe('button');
    expect(chip.props.onClick).toBe(onOpen);
    expect(chip.props['aria-hidden']).toBeUndefined();
    expect(String(chip.props.className)).toContain('cursor-pointer');
  });

  it('применённый период — синий чип, дефолт — серый', () => {
    const applied = chipElement({ period: { from: '2026-01-01', to: '2026-01-31' } });
    expect(String(applied.props.className)).toContain('bg-primary');
    const def = chipElement({ period: null });
    expect(String(def.props.className)).toContain('bg-surface-muted');
  });
});
