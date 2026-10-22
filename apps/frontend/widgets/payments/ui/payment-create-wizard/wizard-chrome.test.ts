import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { TextField } from '@/shared/ui/design';
import { AmountStep } from './amount-step';
import {
  shouldFocusAmountOnMount,
  WizardAmountField,
  WizardAmountFieldView,
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
  focusOnMount?: boolean;
  variant?: unknown;
  title?: unknown;
  inputMode?: unknown;
  pattern?: unknown;
  enterKeyHint?: unknown;
  value?: unknown;
  ref?: unknown;
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

  it('выбранный — белая пилюля с тенью, невыбранный — вторичный текст с шагами hover/active трека', () => {
    const segment = WizardDirectionSegment({
      type: 'income',
      onTypeChange: NOOP,
      ariaLabel: 'Направление платежа',
    });
    const expense = tree(segment).find((el) => el.props.children === 'Расход');
    const income = tree(segment).find((el) => el.props.children === 'Доход');
    expect(String(expense?.props.className)).toContain('text-content-secondary');
    // Без базовой заливки: ни сплошного bg-surface, ни bg-primary — только
    // hover/active-шаги трека (радио-диалект ParticipantRoleSegmented).
    expect(String(expense?.props.className)).not.toMatch(/(^|\s)bg-surface(\s|$)/);
    expect(String(expense?.props.className)).not.toContain('bg-primary');
    expect(String(expense?.props.className)).toContain('hover:bg-surface-muted-hover');
    expect(String(expense?.props.className)).toContain('active:bg-surface-muted-active');
    expect(String(income?.props.className)).toContain('bg-surface');
    expect(String(income?.props.className)).toContain('shadow-');
  });

  it('сегмент во всю ширину на всех ярусах — макет 2913:69741 отменил 232px (решение владельца 22.10)', () => {
    const segment = WizardDirectionSegment({
      type: 'expense',
      onTypeChange: NOOP,
      ariaLabel: 'Направление операции',
    });
    const className = String(segment.props.className);
    expect(className).toContain('w-full');
    expect(className).not.toContain('max-w-[232px]');
    expect(className).not.toContain('desktop:max-w-none');
  });

  it('тень пилюли обрезается треком (overflow-clip) и между пилюлями зазор 2px — как в макете 2913:69741', () => {
    const segment = WizardDirectionSegment({
      type: 'expense',
      onTypeChange: NOOP,
      ariaLabel: 'Направление операции',
    });
    const className = String(segment.props.className);
    expect(className).toContain('overflow-clip');
    expect(className).toContain('gap-[2px]');
    expect(className).toContain('p-[2px]');
    expect(className).toContain('rounded-2xl');
    expect(className).toContain('bg-surface-muted');
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

describe('автоподъём клавиатуры — флаг focusOnMount через шаг суммы (#1151)', () => {
  it('AmountStep: флаг доезжает до WizardAmountField (переход «Далее» шага 4 — жест)', () => {
    const step = AmountStep({
      amountKopecks: undefined,
      onAmountChange: NOOP,
      type: undefined,
      onTypeChange: NOOP,
      focusOnMount: true,
    });
    const field = tree(step).find((el) => el.type === WizardAmountField);
    expect(field?.props.focusOnMount).toBe(true);
  });

  it('AmountStep: без флага focusOnMount не навешивается (черновик и роут — маунты вне жеста)', () => {
    const step = AmountStep({
      amountKopecks: undefined,
      onAmountChange: NOOP,
      type: undefined,
      onTypeChange: NOOP,
    });
    const field = tree(step).find((el) => el.type === WizardAmountField);
    expect(field?.props.focusOnMount).toBeUndefined();
  });
});

describe('WizardAmountFieldView — единый ярус (решение владельца 22.10, макет 2913:69551)', () => {
  it('один бокс Title In на всех ширинах — без дисплейного яруса AmountField', () => {
    const field = WizardAmountFieldView({
      label: 'Сумма',
      buffer: '',
      onInputChange: NOOP,
      onFocus: NOOP,
      onBlur: NOOP,
    });
    const box = tree(field).find((el) => el.type === TextField);
    expect(box).toBeDefined();
    expect(box?.props.variant).toBe('titleIn');
    expect(box?.props.title).toBe('Сумма');
  });

  it('контракт мобильной клавиатуры бывшего дисплея — inputMode/pattern/enterKeyHint на боксе', () => {
    const field = WizardAmountFieldView({
      label: 'Сумма',
      buffer: '2500,5',
      onInputChange: NOOP,
      onFocus: NOOP,
      onBlur: NOOP,
    });
    const box = tree(field).find((el) => el.type === TextField);
    expect(box?.props.inputMode).toBe('decimal');
    expect(box?.props.pattern).toBe('[0-9]*');
    expect(box?.props.enterKeyHint).toBe('done');
    // Группировка разрядов рисуется в значении бокса.
    expect(box?.props.value).toBe('2\u00A0500,5');
  });

  it('маунт-фокус — callback-ref на инпуте бокса (решение на маунте, не проп autoFocus)', () => {
    const mountFocusRef = (): void => undefined;
    const field = WizardAmountFieldView({
      label: 'Сумма',
      buffer: '',
      mountFocusRef,
      onInputChange: NOOP,
      onFocus: NOOP,
      onBlur: NOOP,
    });
    const box = tree(field).find((el) => el.type === TextField);
    expect(box?.props.ref).toBe(mountFocusRef);
  });

  it('shouldFocusAmountOnMount: жест (#1151) или ПК ≥1024 — фокус; мобилка вне жеста — нет', () => {
    expect(shouldFocusAmountOnMount(true, false)).toBe(true);
    expect(shouldFocusAmountOnMount(false, true)).toBe(true);
    expect(shouldFocusAmountOnMount(true, true)).toBe(true);
    expect(shouldFocusAmountOnMount(false, false)).toBe(false);
  });
});

describe('отступы шагов суммы — едины на всех ширинах (макет 2913:69553)', () => {
  it('AmountStep: pt-24/px-24/gap-24 без ярусных вилок', () => {
    const step = AmountStep({
      amountKopecks: undefined,
      onAmountChange: NOOP,
      type: undefined,
      onTypeChange: NOOP,
    });
    const className = String(step.props.className);
    expect(className).toContain('gap-6');
    expect(className).toContain('px-6');
    expect(className).toContain('pt-6');
    expect(className).not.toContain('pt-16');
    expect(className).not.toContain('gap-8');
    expect(className).not.toContain('desktop:');
  });

  it('OperationAmountStep: те же единые отступы', () => {
    const step = OperationAmountStep({
      amountKopecks: undefined,
      onAmountChange: NOOP,
      type: 'expense',
      onTypeChange: NOOP,
    });
    const className = String(step.props.className);
    expect(className).toContain('gap-6');
    expect(className).toContain('pt-6');
    expect(className).not.toContain('desktop:');
  });
});
