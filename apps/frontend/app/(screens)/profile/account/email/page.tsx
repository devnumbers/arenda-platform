import type { Metadata } from 'next';
import { EmailChangeScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Смена почты — Рентли',
  description: 'Подтверждаемая смена электронной почты пользователя',
};

export default function ChangeEmailPage() {
  return <EmailChangeScreen />;
}
