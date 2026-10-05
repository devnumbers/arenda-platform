import { describe, expect, it } from 'vitest';
import { ResendCodeTile } from './resend-code-tile';

/** Плитка без хуков — канон node-env тестов (как button.test.ts):
 * вызов как функция, инспекция дерева без рендера. Гард #1099: loading
 * обязан диспейблить кнопку даже рядом с явным disabled (оператор ??
 * в Button гасит loading, когда передан явный disabled — без ||
 * кнопка повторной отправки остаётся активной в полёте). */
describe('ResendCodeTile', () => {
  it('loading диспейблит кнопку при нулевом таймере (#1099)', () => {
    const tile = ResendCodeTile({ remainingSeconds: 0, loading: true, onResend: () => {} });
    const button = tile.props.children[0];
    expect(button.props.disabled).toBe(true);
    expect(button.props.loading).toBe(true);
  });

  it('кулдаун диспейблит кнопку и рисует подпись таймера', () => {
    const tile = ResendCodeTile({ remainingSeconds: 42, onResend: () => {} });
    const button = tile.props.children[0];
    expect(button.props.disabled).toBe(true);
    expect(tile.props.children[1].props.children[0]).toContain('Запросить новый код можно через');
  });

  it('без loading и кулдауна кнопка активна, подписи нет', () => {
    const tile = ResendCodeTile({ remainingSeconds: 0, onResend: () => {} });
    const button = tile.props.children[0];
    expect(button.props.disabled).toBe(false);
    expect(tile.props.children[1]).toBeFalsy();
  });
});
