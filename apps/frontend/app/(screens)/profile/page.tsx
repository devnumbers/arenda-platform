import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { ProfileOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Профиль — Рентли',
  description: 'Страница профиля пользователя',
};

/** Корень дерева профиля на едином хроме (карта #556, тикет #566): вход —
 * кнопка юзера в хедере («крыло» UserButton), выход — ведущее «Назад»
 * (goBack, фолбэк — дом приложения). Подэкран: заголовок в TopNavTitle,
 * состав прежний (редизайн карточки и меню — отдельное усилие). */
export default function ProfilePage() {
  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.properties} />}>
        <TopNavTitle title="Профиль" />
      </TopNav>
      <PageContent className="px-6">
        <ProfileOverview />
      </PageContent>
    </>
  );
}
