import type { User, UserUpdateCommand } from '@/entities/user';

/** Текстовые поля профиля, редактируемые на экране «Аккаунт» (#593)
 * тихим автосохранением PATCH /me. Почта сюда не входит (#722): она
 * меняется только через подтверждаемый флоу /profile/account/email. */
export type ProfileTextField = 'name' | 'surname' | 'patronymic';

/** Патч одного поля для автосохранения: значение обрезается по краям,
 * неизменённое (после обрезки) не патчится. Контракт бэка (domain.User.
 * UpdatePersonalData): null/отсутствие поля = «не менять», пустая строка
 * = очистить — поэтому пустое значение уходит как "". */
export function profileFieldPatch(
  me: Pick<User, ProfileTextField>,
  field: ProfileTextField,
  rawValue: string,
): UserUpdateCommand | null {
  const value = rawValue.trim();
  if (value === (me[field] ?? '')) {
    return null;
  }
  const patch: UserUpdateCommand = {};
  patch[field] = value;
  return patch;
}
