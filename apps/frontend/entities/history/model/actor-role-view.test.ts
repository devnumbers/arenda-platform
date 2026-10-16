import { describe, expect, it } from 'vitest';

import { actorRoleLabel } from './actor-role-view';

describe('actorRoleLabel', () => {
  it('владелец — «Владелец»', () => {
    expect(actorRoleLabel('owner')).toBe('Владелец');
  });

  it('роли участия читаются словарём shared/model/access (решение владельца 12.10.2026, #1358)', () => {
    expect(actorRoleLabel('full_access')).toBe('Управление');
    expect(actorRoleLabel('viewer')).toBe('Просмотр');
  });
});
