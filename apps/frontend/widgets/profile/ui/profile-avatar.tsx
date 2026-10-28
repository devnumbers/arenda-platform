import type { JSX } from 'react';
import { UserAvatar } from '@/shared/ui/design';

/**
 * Аватар профиля 96px (Figma Category Icon 651:5921, Background=White):
 * канон UserAvatar hero — серый круг с белым кольцом 2.5, Bold/User 52
 * без фото, фото клипается кругом; битое фото откатывается к заглушке
 * (решение #1286). Потребитель передаёт уже готовый URL показа
 * (stage — mePhotoDisplay; версия кэша уже в URL от бэка). Декоративен: имя рядом, у управляемого
 * круга на экране аккаунта aria-label несёт кнопка-обёртка.
 */
export function ProfileAvatar({
  photoUrl,
  photoBlur,
}: {
  readonly photoUrl: string | null;
  /** Blur-кадр серверного фото (ADR 0065 аменд, решение #1594); при
   * staged-превью не передаётся — байты уже на руках. */
  readonly photoBlur?: string | null;
}): JSX.Element {
  return <UserAvatar size="hero" photoUrl={photoUrl} photoBlur={photoBlur} />;
}
