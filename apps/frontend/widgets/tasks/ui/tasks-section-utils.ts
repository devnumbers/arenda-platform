/**
 * Общие помощники секций экранов задач (#499 и лента #523): стабильный
 * ключ секции в списке. Тон секции — канон features/tasks (taskSectionTone).
 */

import type { TaskSection } from '@/features/tasks';

export function sectionKey(section: TaskSection): string {
  return section.kind === 'dated' ? `dated-${section.date}` : section.kind;
}
