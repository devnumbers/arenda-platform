import type { Metadata } from 'next';
import { PhoneChangeScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Изменение телефона — Рентли',
  description: 'Изменение номера телефона пользователя',
};

export default function ChangePhonePage() {
  return <PhoneChangeScreen />;
}
