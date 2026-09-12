import type { Metadata } from 'next';
import { PropertiesSearchScreen } from '@/widgets/properties';

/** Поиск по объектам — страница с поисковой шапкой канона (карта #596,
 * тикет #601; пилюля на хабе «Объекты»). На едином хроме экранов (#565). */
export const metadata: Metadata = {
  title: 'Поиск объектов — Рентли',
};

export default function PropertySearchRoutePage() {
  return <PropertiesSearchScreen />;
}
