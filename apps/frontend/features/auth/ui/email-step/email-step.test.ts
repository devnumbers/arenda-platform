import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { ROUTES } from '@/shared/config/routes';
import { Button, ResendCodeTile } from '@/shared/ui/design';
import { EmailStep } from './EmailStep';

/** Плоское дерево React-элементов шага без рендера: канон тестов фронта —
 * node без DOM (vitest.config.mts); шаг без хуков вызывается как функция
 * и инспектируется поэлементно (канон — phone-step.test.ts; сам
 * resend-code-tile.test.ts дерево не треверсит). SupportRequestButton в
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

function inspect(props: Parameters<typeof EmailStep>[0]): {
  elements: InspectableElement[];
  strings: string[];
} {
  const elements: InspectableElement[] = [];
  const strings: string[] = [];
  flatten(EmailStep(props), elements, strings);
  return { elements, strings };
}

const baseProps = {
  email: 'new-user@example.com',
  onEmailChange: () => {},
  onSubmit: () => {},
  isLoading: false,
} as const;

describe('EmailStep — кулдаун resend-канона', () => {
  it('при таймере вместо Primary «Получить код» — плитка ResendCodeTile с остатком', () => {
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

  it('без таймера — Primary «Получить код» активна при валидной почте, плитки нет', () => {
    const { elements } = inspect({ ...baseProps });

    const submit = elements.find(
      (element) => element.type === Button && element.props.children === 'Получить код',
    );
    expect(submit).toBeDefined();
    expect(submit?.props.disabled).toBe(false);
    expect(elements.find((element) => element.type === ResendCodeTile)).toBeUndefined();
  });
});

describe('EmailStep — consent-футер первого шага', () => {
  it('юртекст согласия с ссылками на лендинг в новой вкладке (переехал с телефонного шага, ADR 0067)', () => {
    const { elements, strings } = inspect(baseProps);

    // Шаг почты — теперь вход в поток (вход = регистрация), consent живёт
    // здесь: прежний носитель — шаг телефона — снесён (#1529).
    const privacy = elements.find(
      (element) => element.type === 'a' && element.props.href === ROUTES.landingPrivacy,
    );
    const terms = elements.find(
      (element) => element.type === 'a' && element.props.href === ROUTES.landingTerms,
    );
    expect(privacy).toBeDefined();
    expect(terms).toBeDefined();
    expect(privacy?.props.target).toBe('_blank');
    expect(terms?.props.target).toBe('_blank');

    const consent = strings.join(' ');
    // Кнопка шага названа по имени — согласие привязано к «Получить код»,
    // а не к прежнему «Войти».
    expect(consent).toContain('Нажимая кнопку «Получить код»');
    expect(consent).toContain('политику конфиденциальности');
    expect(consent).toContain('обработку персональных данных');
  });
});
