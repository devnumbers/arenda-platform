import { describe, expect, it } from 'vitest';
import { PaymentsGlobalPlaceholder } from './payments-global-screen';

/** Пустая секция хаба «Платежей» кликается целиком (решение владельца
 * 28.10): карточка-плейсхолдер — одна кнопка с действием заголовка секции,
 * канон взаимодействия — как у заголовков-кнопок (pressable, hover,
 * focus-ring). Проводка onOpen в телах секций — экранная приёмка
 * /ui-walkthrough. */

const noop = (): void => {};

describe('PaymentsGlobalPlaceholder — пустая секция хаба кликается целиком', () => {
  it('карточка — кнопка с действием секции', () => {
    const placeholder = PaymentsGlobalPlaceholder({
      image: '/images/payments/due-empty.webp',
      onOpen: noop,
      children: 'Здесь будут платежи на оплату',
    });
    expect(placeholder.type).toBe('button');
    expect(placeholder.props.onClick).toBe(noop);
  });

  it('кнопка несёт канон взаимодействия заголовков и геометрию карточки', () => {
    const placeholder = PaymentsGlobalPlaceholder({
      image: '/images/payments/due-empty.webp',
      onOpen: noop,
      children: 'Здесь будут платежи на оплату',
    });
    const className = String(placeholder.props.className);
    expect(className).toContain('pressable');
    expect(className).toContain('cursor-pointer');
    expect(className).toContain('text-left');
    expect(className).toContain('hover:opacity-80');
    expect(className).toContain('focus-visible:ring-4');
    expect(className).toContain('rounded-card');
    expect(className).toContain('bg-surface-muted');
    expect(className).toContain('min-h-[96px]');
    expect(className).toContain('pl-7');
  });
});
