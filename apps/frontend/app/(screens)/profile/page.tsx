import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { ProfileHub } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Профиль — Рентли',
  description: 'Страница профиля пользователя',
};

/** Хаб профиля в новом дизайне (карта #591, тикет #592; Figma 1903-38340 /
 * 1786-31288): хаб-экран без ведущего «Назад» — вход «крылом» UserButton,
 * заголовок «Профиль» 16/18 в баре по центру (макет), крылья и на мобайле.
 * Подэкраны /profile* ссылаются на него фолбэком «Назад». */
export default function ProfilePage() {
  return (
    <>
      <TopNav mobileWings>
        <TopNavTitle title="Профиль" />
      </TopNav>
      <PageContent>
        <ProfileHub />
      </PageContent>
    </>
  );
}
