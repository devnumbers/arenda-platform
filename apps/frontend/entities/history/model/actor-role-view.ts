/**
 * Подпись роли актёра в шапке группы ленты: роль участия — словарь
 * shared/model/access (full_access читается «Управление», #1358),
 * владелец журналом пишется ролью owner — он не
 * membership, подпись живёт здесь.
 */

import { ACCESS_ROLE_LABELS, type SharedAccessRole } from '@/shared/model/access';

import type { HistoryActorRole } from './types';

export function actorRoleLabel(role: HistoryActorRole): string {
  return role === 'owner' ? 'Владелец' : ACCESS_ROLE_LABELS[role satisfies SharedAccessRole];
}
