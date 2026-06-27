import type { Metadata } from 'next';
import { ProfileOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Профиль — Arenda Platform',
  description: 'Страница профиля пользователя',
};

export default function ProfilePage() {
  return <ProfileOverview />;
}
