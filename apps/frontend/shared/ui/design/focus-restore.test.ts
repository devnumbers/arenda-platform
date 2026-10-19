import { describe, expect, it, vi } from 'vitest';
import { createFocusRestore, focusRestoreContentProps } from './focus-restore';

/**
 * Контракт безкольцевого возврата фокуса (залипшее :focus-visible на
 * триггере после закрытия модалок; решение владельца 19.10 — контракт
 * «никогда», даже после Esc): capture фиксирует сфокусированный элемент
 * (или явный триггер в момент его события), restore глушит
 * preventDefault'ом дефолт Radix (тот вернул бы фокус голым focus() —
 * эвристика браузера считает его клавиатурным и рисует кольцо) и
 * возвращает фокус сам — с литералом {preventScroll, focusVisible:false},
 * который браузер (Chromium 145+, Firefox 104+, Safari 18.4+) обязывает
 * не матчить :focus-visible. События и элементы — литералы-срезы, ни
 * React, ни DOM (прецедент — list-row-keyboard.test.ts). Живая эвристика
 * :focus-visible проверяется в e2e (modal-focus-ring.spec.ts, реальный
 * Chromium).
 */

const RINGLESS_FOCUS = { preventScroll: true, focusVisible: false } as const;

const focusableElement = (): Element =>
  ({ focus: vi.fn(), blur: vi.fn() }) as unknown as Element;

const focusedElement = (element: Element): (() => Element | null) => () => element;

const asFocusable = (element: Element): { focus: ReturnType<typeof vi.fn>; blur: ReturnType<typeof vi.fn> } =>
  element as unknown as { focus: ReturnType<typeof vi.fn>; blur: ReturnType<typeof vi.fn> };

const event = () => ({ preventDefault: vi.fn() });

describe('createFocusRestore: capture/restore возвращают фокус хозяина без кольца', () => {
  it('capture без аргумента берет активный элемент, restore фокусирует его с литералом {preventScroll, focusVisible:false}', () => {
    const trigger = focusableElement();
    const restore = createFocusRestore({ activeElement: focusedElement(trigger) });
    const close = event();

    restore.capture();
    restore.restore(close);

    expect(asFocusable(trigger).focus).toHaveBeenCalledWith(RINGLESS_FOCUS);
    expect(close.preventDefault).toHaveBeenCalledOnce();
  });

  it('capture с явным элементом (триггер в момент его события) пинит именно его', () => {
    const host = focusableElement();
    const elsewhere = focusableElement();
    const restore = createFocusRestore({ activeElement: focusedElement(elsewhere) });
    const close = event();

    restore.capture(host);
    restore.restore(close);

    expect(asFocusable(host).focus).toHaveBeenCalledWith(RINGLESS_FOCUS);
    expect(asFocusable(elsewhere).focus).not.toHaveBeenCalled();
  });

  it('хозяин уже в фокусе (vaul гасит фокус-вход в шит): blur перед свежим фокусом — подавление на новом событии', () => {
    const trigger = focusableElement();
    const restore = createFocusRestore({ activeElement: focusedElement(trigger) });
    const close = event();

    restore.capture();
    restore.restore(close);

    const stub = asFocusable(trigger);
    expect(stub.blur).toHaveBeenCalledOnce();
    expect(stub.focus).toHaveBeenCalledWith(RINGLESS_FOCUS);
    // Порядок существен: подавление привязано к фокус-событию после blur.
    const [blurOrder] = stub.blur.mock.invocationCallOrder;
    const [focusOrder] = stub.focus.mock.invocationCallOrder;
    expect(
      blurOrder !== undefined && focusOrder !== undefined && blurOrder < focusOrder,
      'blur должен отыграть раньше фокуса',
    ).toBe(true);
  });
});

describe('createFocusRestore: захвата нет — дефолт Radix всё равно заглушен', () => {
  it('нефокусируемый элемент не захватывается: restore гасит дефолт, фокус не ставит', () => {
    const restore = createFocusRestore({ activeElement: focusedElement({} as unknown as Element) });
    const close = event();

    restore.capture();
    restore.restore(close);

    expect(close.preventDefault).toHaveBeenCalledOnce();
  });

  it('активного элемента нет (программное открытие): restore гасит дефолт, фокус не ставит', () => {
    const restore = createFocusRestore({ activeElement: () => null });
    const close = event();

    restore.capture();
    restore.restore(close);

    expect(close.preventDefault).toHaveBeenCalledOnce();
  });
});

describe('createFocusRestore: жизненный цикл поверхностей', () => {
  it('повторный capture перезахватывает — восстанавливается последний хозяин', () => {
    const first = focusableElement();
    const second = focusableElement();
    let active: Element | null = first;
    const restore = createFocusRestore({ activeElement: () => active });
    const close = event();

    restore.capture();
    restore.restore(close);
    active = second;
    restore.capture();
    restore.restore(close);

    expect(asFocusable(second).focus).toHaveBeenCalledTimes(1);
    expect(asFocusable(second).focus).toHaveBeenCalledWith(RINGLESS_FOCUS);
    // first — реставратор только своего первого цикла: будь захват
    // устаревшим, second не получил бы ни одного вызова.
    expect(asFocusable(first).focus).toHaveBeenCalledTimes(1);
  });

  it('захват сбрасывается восстановлением — restore без capture фокус не ставит', () => {
    const trigger = focusableElement();
    const restore = createFocusRestore({ activeElement: focusedElement(trigger) });
    const close = event();

    restore.restore(close);

    expect(asFocusable(trigger).focus).not.toHaveBeenCalled();
    expect(close.preventDefault).toHaveBeenCalledOnce();
  });
});

describe('focusRestoreContentProps: шов Content-поверхностей Radix', () => {
  it('onOpenAutoFocus захватывает и не preventDefault — фокус внутрь двигает Radix сам', () => {
    const trigger = focusableElement();
    const restore = createFocusRestore({ activeElement: focusedElement(trigger) });
    const { onOpenAutoFocus, onCloseAutoFocus } = focusRestoreContentProps(restore);
    const open = event();
    const close = event();

    onOpenAutoFocus(open);
    onCloseAutoFocus(close);

    expect(open.preventDefault).not.toHaveBeenCalled();
    expect(asFocusable(trigger).focus).toHaveBeenCalledWith(RINGLESS_FOCUS);
  });
});
