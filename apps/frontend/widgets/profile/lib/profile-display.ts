import type { User } from '@/entities/user';

/** Имя на хабе профиля (тикет #592, Figma 1786-31288): имя + фамилия из
 * /me, иначе «Пользователь». */
export function getProfileDisplayName(user: Pick<User, 'name' | 'surname'>): string {
  const parts = [user.name, user.surname].filter(Boolean);
  const joined = parts.join(' ');
  return joined.length > 0 ? joined : 'Пользователь';
}
