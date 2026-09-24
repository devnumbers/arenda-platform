import { describe, expect, it } from 'vitest';

import { actorRoleLabel } from './actor-role-view';

describe('actorRoleLabel', () => {
  it('владелец — «Владелец»', () => {
    expect(actorRoleLabel('owner')).toBe('Владелец');
  });

  it('роли участия читаются словарём shared/model/access (решение чарта #692)', () => {
    expect(actorRoleLabel('full_access')).toBe('Редактирование');
    expect(actorRoleLabel('viewer')).toBe('Просмотр');
  });
});
