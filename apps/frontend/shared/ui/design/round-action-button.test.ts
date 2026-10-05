import { describe, expect, it } from 'vitest';
import { RoundActionButton } from './round-action-button';

/** Канон node-env тестов (CODING_STANDARDS.md § Testing): RoundActionButton
 * без хуков вызывается как функция, дерево инспектируется без рендера. */
describe('RoundActionButton при loading', () => {
  it('кнопка дизейблится и помечается aria-busy', () => {
    const button = RoundActionButton({ icon: 'i', caption: 'Оплатить', loading: true });
    expect(button.props.disabled).toBe(true);
    expect(button.props['aria-busy']).toBe(true);
  });

  it('инвариант #1119: loading глушит и при явном disabled=false (disabled || loading)', () => {
    const button = RoundActionButton({
      icon: 'i',
      caption: 'Оплатить',
      loading: true,
      disabled: false,
    });
    expect(button.props.disabled).toBe(true);
  });

  it('без loading кнопка остаётся кликабельной — disabled не включается', () => {
    const button = RoundActionButton({ icon: 'i', caption: 'Оплатить' });
    expect(button.props.disabled).toBeFalsy();
  });
});
