import { describe, expect, it } from 'vitest';

import { OWN_USER_NAME_SUFFIX } from '@/shared/lib/display-name';

import { MeSuffix } from './me-suffix';

describe('MeSuffix — серый суффикс своей строки (канон шита фильтров #710, извлечение #1356)', () => {
  it('единственная точка рисования: span text-content-tertiary с канонным текстом', () => {
    const suffix = MeSuffix();
    expect(suffix.type).toBe('span');
    expect(suffix.props.className).toBe('text-content-tertiary');
    expect(suffix.props.children).toBe(OWN_USER_NAME_SUFFIX);
  });
});
