import { pluralize } from '@/shared/lib/pluralize';

/** Подзаголовок-счётчик экрана «Архивные объекты» (тикет #587, макет
 * 1603:92102): «1 объект / 2 объекта / 5 объектов». */
export function archiveCountLabel(count: number): string {
  return `${count} ${pluralize(count, 'объект', 'объекта', 'объектов')}`;
}
