import type { Metadata } from 'next';
import { ContactBookSearchScreen } from '@/widgets/contacts';

/** Поиск по плоской книге контактов — отдельная страница с поисковой
 * шапкой 1:1 как у книги объекта (#508; решение владельца 2026-09-05). */
export const metadata: Metadata = {
  title: 'Поиск контактов — Рентли',
};

export default function ContactSearchRoutePage() {
  return <ContactBookSearchScreen />;
}
