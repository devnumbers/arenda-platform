import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { AmountStep } from './amount-step';
import {
  WizardAmountField,
  WizardDirectionSegment,
  WizardHeading,
} from './wizard-chrome';
import { OperationAmountStep } from '../operation-create-wizard/operation-amount-step';

/** Плоское дерево React-элементов без рендера: канон тестов фронта —
 * node без DOM (vitest.config.mts); компоненты без хуков вызываются как
 * функции и инспектируются (канон button.test.ts). */
type InspectableProps = {
  children?: ReactNode;
  className?: string;
  role?: string;
  'aria-label'?: string;
  'aria-checked'?: boolean;
  onClick?: () => void;
  type?: unknown;
  ariaLabel?: string;
};

function flatten(
  node: ReactNode,
  elements: ReactElement<InspectableProps>[],
): void {
  if (typeof node === 'string') {
    return;
  }
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, elements);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    // числа, false/undefined (условный JSX), прочие листья
    return;
  }
  const element = node as ReactElement<InspectableProps>;
  elements.push(element);
  flatten(element.props.children, elements);
}

function tree(node: ReactNode): ReactElement<InspectableProps>[] {
  const elements: ReactElement<InspectableProps>[] = [];
  flatten(node, elements);
  return elements;
}

const NOOP = (): void => undefined;

describe('WizardHeading — вариант H1/600 хедера выбора категории (#1152)', () => {
  it('дефолт — прежний H3 20/24 (text-xl leading-6), подзаголовок на месте', () => {
    const heading = WizardHeading({ title: 'Назовите платеж', subtitle: 'Подсказка' });
    const h1 = tree(heading).find((el) => el.type === 'h1');
    expect(String(h1?.props.className)).toContain('text-xl');
    expect(String(h1?.props.className)).toContain('leading-6');
    expect(tree(heading).some((el) => el.type === 'p')).toBe(true);
  });

  it('variant h1 — 28/32 H1/600 (text-2xl leading-8), тот же вес и токен текста', () => {
    const heading = WizardHeading({
      title: 'Выберите категорию платежа',
      variant: 'h1',
    });
    const h1 = tree(heading).find((el) => el.type === 'h1');
    expect(String(h1?.props.className)).toContain('text-2xl');
    expect(String(h1?.props.className)).toContain('leading-8');
    expect(String(h1?.props.className)).toContain('font-semibold');
    expect(String(h1?.props.className)).toContain('text-content');
  });

  it('variant h1 без subtitle — подзаголовка нет («Без описания», спека #1149)', () => {
    const heading = WizardHeading({
      title: 'Выберите категорию платежа',
      variant: 'h1',
    });
    expect(tree(heading).some((el) => el.type === 'p')).toBe(false);
  });

  it('отступы блока не менялись: 24 по бокам и сверху, заголовок — один h1', () => {
    const heading = WizardHeading({ title: 'Категория', variant: 'h1' });
    expect(String(heading.props.className)).toContain('px-6');
    expect(String(heading.props.className)).toContain('pt-6');
    expect(tree(heading).filter((el) => el.type === 'h1')).toHaveLength(1);
  });
});

describe('WizardDirectionSegment — общий сегмент «Расход/Доход» шагов суммы', () => {
  it('группа из двух радио в порядке Расход → Доход, aria-label насквозь', () => {
    const segment = WizardDirectionSegment({
      type: 'expense',
      onTypeChange: NOOP,
      ariaLabel: 'Направление платежа',
    });
    expect(segment.props.role).toBe('radiogroup');
    expect(segment.props['aria-label']).toBe('Направление платежа');

    const radios = tree(segment).filter((el) => el.props.role === 'radio');
    expect(radios.map((el) => el.props.children)).toEqual(['Расход', 'Доход']);
    expect(radios.map((el) => el.props['aria-checked'])).toEqual([true, false]);
  });

  it('выбранный — белая пилюля с тенью, невыбранный — вторичный текст', () => {
    const segment = WizardDirectionSegment({
      type: 'income',
      onTypeChange: NOOP,
      ariaLabel: 'Направление платежа',
    });
    const expense = tree(segment).find((el) => el.props.children === 'Расход');
    const income = tree(segment).find((el) => el.props.children === 'Доход');
    expect(String(expense?.props.className)).toContain('text-content-secondary');
    expect(String(expense?.props.className)).not.toContain('bg-surface');
    expect(String(income?.props.className)).toContain('bg-surface');
    expect(String(income?.props.className)).toContain('shadow-');
  });

  it('контейнер 232px на дисплейном ярусе, вся колонка только от 1024 (граница desktop:, не md:)', () => {
    const segment = WizardDirectionSegment({
      type: 'expense',
      onTypeChange: NOOP,
      ariaLabel: 'Направление операции',
    });
    expect(String(segment.props.className)).toContain('max-w-[232px]');
    expect(String(segment.props.className)).toContain('desktop:max-w-none');
    expect(String(segment.props.className)).not.toContain('md:max-w-none');
  });

  it('клик по радио вызывает onTypeChange с этой опцией', () => {
    const onTypeChange = vi.fn();
    const segment = WizardDirectionSegment({
      type: 'expense',
      onTypeChange,
      ariaLabel: 'Направление операции',
    });
    const income = tree(segment).find((el) => el.props.children === 'Доход');
    income?.props.onClick?.();
    expect(onTypeChange).toHaveBeenCalledWith('income');
  });
});

describe('шаг суммы визарда платежа — один в один с операционным (дополнение #1005)', () => {
  const step = AmountStep({
    amountKopecks: undefined,
    onAmountChange: NOOP,
    type: undefined,
    onTypeChange: NOOP,
  });

  it('без заголовка «Сумма платежа» — как у операции', () => {
    // WizardHeading — единственный источник h1 на шаге; снесён вместе
    // с импортом.
    expect(tree(step).some((el) => el.type === 'h1')).toBe(false);
  });

  it('общее денежное поле и общий сегмент; до явного выбора подсвечен дефолтный «Доход»', () => {
    const elements = tree(step);
    expect(elements.some((el) => el.type === WizardAmountField)).toBe(true);

    const segment = elements.find((el) => el.type === WizardDirectionSegment);
    expect(segment).toBeDefined();
    expect(segment?.props.type).toBe('income');
    expect(segment?.props.ariaLabel).toBe('Направление платежа');
  });

  it('явный выбор доезжает до сегмента без дефолта', () => {
    const explicit = AmountStep({
      amountKopecks: 250000,
      onAmountChange: NOOP,
      type: 'expense',
      onTypeChange: NOOP,
    });
    const segment = tree(explicit).find((el) => el.type === WizardDirectionSegment);
    expect(segment?.props.type).toBe('expense');
  });
});

describe('шаг суммы визарда операции — общий сегмент', () => {
  it('сегмент с aria-label «Направление операции» и текущим направлением', () => {
    const step = OperationAmountStep({
      amountKopecks: undefined,
      onAmountChange: NOOP,
      type: 'expense',
      onTypeChange: NOOP,
    });
    const segment = tree(step).find((el) => el.type === WizardDirectionSegment);
    expect(segment).toBeDefined();
    expect(segment?.props.type).toBe('expense');
    expect(segment?.props.ariaLabel).toBe('Направление операции');
  });
});
