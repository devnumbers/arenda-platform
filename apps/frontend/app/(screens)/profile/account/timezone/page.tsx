import type { Metadata } from 'next';
import { TimezonePickerScreen } from '@/widgets/profile';

/** Пикер часового пояса (карта #591, тикет #594) — страница-маршрут с
 * поисковой шапкой; на едином хроме экранов (#565). */
export const metadata: Metadata = {
  title: 'Часовой пояс — Рентли',
};

export default function TimezonePickerRoutePage() {
  return <TimezonePickerScreen />;
}
