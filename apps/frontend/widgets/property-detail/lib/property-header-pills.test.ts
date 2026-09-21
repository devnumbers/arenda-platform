import { describe, expect, it } from 'vitest';
import { propertyHeaderPills } from './property-header-pills';

describe('propertyHeaderPills — пилюли шапки детали (#773)', () => {
  const viewerAccess = { role: 'viewer' } as const;
  const editorAccess = { role: 'full_access' } as const;

  it('архивный чужой: «В архиве» рядом с пилюлей роли — признак архива по deep-link', () => {
    expect(
      propertyHeaderPills({ status: 'archived', access: viewerAccess }),
    ).toEqual([{ kind: 'archived' }, { kind: 'access', role: 'viewer' }]);
  });

  it('архивный чужой с полным доступом: роль в пилюле — грант, архив объясняет read-only', () => {
    expect(
      propertyHeaderPills({ status: 'archived', access: editorAccess }),
    ).toEqual([{ kind: 'archived' }, { kind: 'access', role: 'full_access' }]);
  });

  it('архивный свой: только «В архиве» — владелец пилюли роли не получает', () => {
    expect(propertyHeaderPills({ status: 'archived', access: undefined })).toEqual([
      { kind: 'archived' },
    ]);
    expect(
      propertyHeaderPills({ status: 'archived', access: { role: 'owner' } }),
    ).toEqual([{ kind: 'archived' }]);
  });

  it('активный чужой: пилюля роли без архивной; активный свой — без пилюль', () => {
    expect(propertyHeaderPills({ status: 'active', access: viewerAccess })).toEqual([
      { kind: 'access', role: 'viewer' },
    ]);
    expect(propertyHeaderPills({ status: 'maintenance', access: undefined })).toEqual([]);
  });

  it('объект не загружен — пилюль нет', () => {
    expect(propertyHeaderPills(undefined)).toEqual([]);
  });
});
