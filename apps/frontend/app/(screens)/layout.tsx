import { ScreenLayout } from '@/widgets/screen-layout';

/** Оболочка новых экранов (#460): маршруты этой группы рендерятся без
 * Sidebar/BottomNav старого кабинета — см. ScreenLayout. */
export default function ScreensLayout({ children }: { children: React.ReactNode }) {
  return <ScreenLayout>{children}</ScreenLayout>;
}
