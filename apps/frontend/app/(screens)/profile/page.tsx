import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { ProfileOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Профиль — Рентли',
  description: 'Страница профиля пользователя',
};

/** Корень дерева профиля на едином хроме (карта #556, тикет #566): вход —
 * кнопка юзера в хедере («крыло» UserButton), выход — ведущее «Назад»
 * (goBack, фолбэк — дом приложения). Каркас — SubScreenShell (экстракция
 * #568), состав прежний (редизайн карточки и меню — отдельное усилие). */
export default function ProfilePage() {
  return (
    <>
      <SubScreenShell title="Профиль" fallbackHref={ROUTES.properties}>
        <ProfileOverview />
      </SubScreenShell>
    </>
  );
}
