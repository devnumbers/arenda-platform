import type { User, UserUpdateCommand } from '@/entities/user';

/** Текстовые поля профиля, редактируемые на экране «Аккаунт» (#593)
 * тихим автосохранением PATCH /me; телефон и часовой пояс — не здесь. */
export type ProfileTextField = 'name' | 'surname' | 'patronymic' | 'email';

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/** Непустая почта должна быть адресом; пустая трактуется отдельно
 * (бэк очистку почты не принимает — см. AccountScreen). */
export function isEmailValid(email: string): boolean {
  return email === '' || EMAIL_REGEX.test(email);
}

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
