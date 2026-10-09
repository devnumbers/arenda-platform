import type { JSX } from 'react';
import { UserAvatar } from '@/shared/ui/design';

/**
 * Аватар профиля 96px (Figma Category Icon 651:5921, Background=White):
 * канон UserAvatar hero — серый круг с белым кольцом 2.5, Bold/User 52
 * без фото, фото клипается кругом; битое фото откатывается к заглушке
 * (решение #1286). Потребитель передаёт уже готовый URL показа
 * (бастер/stage — mePhotoDisplay). Декоративен: имя рядом, у управляемого
 * круга на экране аккаунта aria-label несёт кнопка-обёртка.
 */
export function ProfileAvatar({ photoUrl }: { readonly photoUrl: string | null }): JSX.Element {
  return <UserAvatar size="hero" photoUrl={photoUrl} />;
}
