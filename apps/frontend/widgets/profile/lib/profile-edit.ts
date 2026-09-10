import type { User, UserUpdateCommand } from '@/entities/user';

/** Текстовые поля профиля, редактируемые на экране «Аккаунт» (#593)
 * тихим автосохранением PATCH /me; телефон и часовой пояс — не здесь. */
export type ProfileTextField = 'name' | 'surname' | 'patronymic' | 'email';

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/** Пустая почта валидна — поле необязательное (очистка сохраняет null). */
export function isEmailValid(email: string): boolean {
  return email === '' || EMAIL_REGEX.test(email);
}

/** Патч одного поля для автосохранения: значение обрезается по краям,
 * пустое превращается в null, неизменённое (после обрезки) не патчится. */
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
  patch[field] = value === '' ? null : value;
  return patch;
}
