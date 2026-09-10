import type { Metadata } from 'next';
import { PropertiesSearchScreen } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Поиск объектов — Рентли',
  description: 'Поиск по объектам',
};

export default function PropertiesSearchRoutePage() {
  return <PropertiesSearchScreen />;
}
