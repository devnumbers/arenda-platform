import type { Metadata } from 'next';
import { ContactBookScreen } from '@/widgets/contacts';

/** Плоская книга контактов (глобальная страница, макеты 1726:65083/65136/
 * 85937). Оболочка кабинета (лого-хедер, TabBar/сайдбар) — из layout
 * группы (cabinet); точка входа — сайдбар десктопа. */
export const metadata: Metadata = {
  title: 'Контакты — Рентли',
  description: 'Книга контактов',
};

export default function ContactsRoutePage() {
  return <ContactBookScreen />;
}
