import type { JSX } from 'react';

import { OWN_USER_NAME_SUFFIX } from '@/shared/lib/display-name';

/**
 * Серый суффикс своей строки «(Вы)» — единственная точка рисования
 * (канон шита фильтров #710, извлечён #1356): text-content-tertiary
 * поверх канонного имени, вплотную к нему. Базу имени даёт
 * displayOwnUserName (shared/lib/display-name), текст суффикса —
 * OWN_USER_NAME_SUFFIX (его же строкой берёт aria-label своей строки).
 */
export function MeSuffix(): JSX.Element {
  return <span className="text-content-tertiary">{OWN_USER_NAME_SUFFIX}</span>;
}
