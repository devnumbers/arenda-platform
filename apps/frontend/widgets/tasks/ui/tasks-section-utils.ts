/**
 * Общие помощники секций экранов задач (#499 и лента #523): тон строки по
 * типу секции и стабильный ключ секции в списке.
 */

import type { TaskSection, TaskSectionKind } from '@/features/tasks';
import type { TaskRowTone } from './task-row';

export function sectionTone(kind: TaskSectionKind): TaskRowTone {
  if (kind === 'overdue') {
    return 'danger';
  }
  if (kind === 'today' || kind === 'tomorrow') {
    return 'primary';
  }
  return 'muted';
}

export function sectionKey(section: TaskSection): string {
  return section.kind === 'dated' ? `dated-${section.date}` : section.kind;
}
