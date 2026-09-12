import type { Metadata } from 'next';
import { PropertiesArchiveScreen } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Архивные объекты — Рентли',
  description: 'Архивные объекты',
};

export default function PropertiesArchivePage() {
  return <PropertiesArchiveScreen />;
}
