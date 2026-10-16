import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavTitle } from '@/shared/ui/design';
import { ProfileHub } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Профиль — Рентли',
  description: 'Страница профиля пользователя',
};

/** Хаб профиля в новом дизайне (карта #591, тикет #592; Figma 1903-38340 /
 * 1786-31288): шапка — анатомия подэкрана без «Назад» (решение владельца
 * 02.10, образец — /profile/devices): крылья только на ПК, заголовок
 * «Профиль» 16/18 в баре по центру на всех ярусах. Ниже ПК вход в профиль —
 * TabBar («Еще») и крылья других хабов, на ПК — крыло UserButton.
 * Подэкраны /profile* ссылаются на него фолбэком «Назад». */
export default function ProfilePage() {
  return (
    <>
      <TopNav hideWingsBelowDesktop>
        <TopNavTitle title="Профиль" />
      </TopNav>
      <PageContent>
        <ProfileHub />
      </PageContent>
    </>
  );
}
