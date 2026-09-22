import { pluralize } from '@/shared/lib/pluralize';

/** Подпись счётчика карточки «Ваши участники» (макет 2008-47013):
 * «5 участников»; ноль — «Нет участников» (макет 1967-86441). */
export function participantsCountLabel(count: number): string {
  return countOrNone(count, 'Нет участников', 'участник', 'участника', 'участников');
}

/** Подпись счётчика карточки «Объекты пользователей» (макет 2008-47013):
 * «3 объекта»; ноль — «Нет объектов» (макет 1967-86441). */
export function sharedPropertiesCountLabel(count: number): string {
  return countOrNone(count, 'Нет объектов', 'объект', 'объекта', 'объектов');
}

function countOrNone(
  count: number,
  noneLabel: string,
  one: string,
  few: string,
  many: string,
): string {
  if (count <= 0) {
    return noneLabel;
  }

  return `${count} ${pluralize(count, one, few, many)}`;
}
