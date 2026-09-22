import { describe, expect, it } from 'vitest';

import { ACCESS_ROLE_ICON_NAMES, ACCESS_ROLE_LABELS } from './access';

/** Канон отображения ролей (решение чарта карты #692 — роли только
 * отображение): full_access читается «Редактирование», viewer —
 * «Просмотр»; иконки — Icon/S/Edit и Icon/S/Eye. */
describe('ACCESS_ROLE_LABELS / ACCESS_ROLE_ICON_NAMES', () => {
  it('каждой роли соответствуют и подпись, и имя иконки', () => {
    expect(ACCESS_ROLE_LABELS.viewer).toBe('Просмотр');
    expect(ACCESS_ROLE_LABELS.full_access).toBe('Редактирование');
    expect(ACCESS_ROLE_ICON_NAMES.viewer).toBe('eye');
    expect(ACCESS_ROLE_ICON_NAMES.full_access).toBe('edit');
  });

  it('словари покрывают все SharedAccessRole без лишних ключей', () => {
    expect(Object.keys(ACCESS_ROLE_LABELS).sort()).toEqual(['full_access', 'viewer']);
    expect(Object.keys(ACCESS_ROLE_ICON_NAMES).sort()).toEqual(['full_access', 'viewer']);
  });
});
