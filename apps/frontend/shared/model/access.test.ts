import { describe, expect, it } from 'vitest';

import { ACCESS_ROLE_ICON_NAMES, ACCESS_ROLE_LABELS } from './access';

/** Канон отображения ролей (решение владельца 12.10.2026, #1358 — было
 * «Редактирование» по чарту карты #692): full_access читается «Управление»,
 * viewer — «Просмотр»; иконки — Icon/S/Edit и Icon/S/Eye. */
describe('ACCESS_ROLE_LABELS / ACCESS_ROLE_ICON_NAMES', () => {
  it('каждой роли соответствуют и подпись, и имя иконки', () => {
    expect(ACCESS_ROLE_LABELS.viewer).toBe('Просмотр');
    expect(ACCESS_ROLE_LABELS.full_access).toBe('Управление');
    expect(ACCESS_ROLE_ICON_NAMES.viewer).toBe('eye');
    expect(ACCESS_ROLE_ICON_NAMES.full_access).toBe('edit');
  });

  it('словари покрывают все SharedAccessRole без лишних ключей', () => {
    expect(Object.keys(ACCESS_ROLE_LABELS).sort()).toEqual(['full_access', 'viewer']);
    expect(Object.keys(ACCESS_ROLE_ICON_NAMES).sort()).toEqual(['full_access', 'viewer']);
  });
});
