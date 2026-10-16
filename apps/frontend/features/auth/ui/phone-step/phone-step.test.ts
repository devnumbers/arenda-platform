import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { Button, ResendCodeTile } from '@/shared/ui/design';
import { PhoneStep } from './PhoneStep';

/** Плоское дерево React-элементов шага без рендера: канон тестов фронта —
 * node без DOM (vitest.config.mts); шаг без хуков вызывается как функция
 * и инспектируется поэлементно (как properties-sort-row.test.ts и
 * shared/ui/design/resend-code-tile.test.ts). SupportRequestButton в
 * дереве — лист-элемент: внутрь (useState модалки) вне рендера не
 * заходим. */
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
    // числа, false/undefined (условный JSX), прочие листья
    return;
  }
  const element = node as InspectableElement;
  into.push(element);
  flatten(element.props.children, into, strings);
}

function inspect(props: Parameters<typeof PhoneStep>[0]): {
  elements: InspectableElement[];
  strings: string[];
} {
  const elements: InspectableElement[] = [];
  const strings: string[] = [];
  flatten(PhoneStep(props), elements, strings);
  return { elements, strings };
}

const baseProps = {
  phone: '+7 (926) 111-22-33',
  onPhoneChange: () => {},
  onSendPhone: () => {},
  isLoading: false,
} as const;

describe('PhoneStep — кулдаун resend-канона', () => {
  it('при таймере вместо Primary «Войти» — плитка ResendCodeTile с остатком', () => {
    const { elements, strings } = inspect({ ...baseProps, resendTimer: 42 });

    // Решение владельца 12.10 (макеты 2349:67594/67633): на время кулдауна
    // кнопка шага меняется на плитку resend-канона — Secondary «Отправить
    // новый код», погашенная, с подписью таймера.
    const tile = elements.find((element) => element.type === ResendCodeTile);
    expect(tile).toBeDefined();
    expect(tile?.props.remainingSeconds).toBe(42);
    expect(typeof tile?.props.onResend).toBe('function');

    // Primary-кнопки в дереве нет вовсе (плитка — лист-элемент, её
    // внутренняя кнопка канона наружу не торчит).
    expect(elements.find((element) => element.type === Button)).toBeUndefined();

    // Однострочная подпись прежнего кулдауна не соседствует с плиткой —
    // подпись таймера теперь часть плитки.
    expect(strings.some((text) => text.startsWith('Запросить код можно'))).toBe(false);
  });

  it('без таймера — Primary «Войти» активна при валидном номере, плитки нет', () => {
    const { elements } = inspect({ ...baseProps });

    const submit = elements.find(
      (element) => element.type === Button && element.props.children === 'Войти',
    );
    expect(submit).toBeDefined();
    expect(submit?.props.disabled).toBe(false);
    expect(elements.find((element) => element.type === ResendCodeTile)).toBeUndefined();
  });
});
