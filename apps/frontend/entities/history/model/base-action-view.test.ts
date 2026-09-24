import { describe, expect, it } from 'vitest';

import { baseActionLabel, baseActionTone } from './base-action-view';

describe('baseActionTone', () => {
  it('мапит словарь основных действий на тона иконки (CONTEXT: добавление зелёный, изменение жёлтый, выполнение синий, удаление красный)', () => {
    expect(baseActionTone('added')).toBe('success');
    expect(baseActionTone('changed')).toBe('warning');
    expect(baseActionTone('completed')).toBe('primary');
    expect(baseActionTone('deleted')).toBe('danger');
  });
});

describe('baseActionLabel', () => {
  it('несёт канонические подписи групп фильтра', () => {
    expect(baseActionLabel('added')).toBe('Добавление');
    expect(baseActionLabel('changed')).toBe('Изменение');
    expect(baseActionLabel('completed')).toBe('Выполнение');
    expect(baseActionLabel('deleted')).toBe('Удаление');
  });
});
