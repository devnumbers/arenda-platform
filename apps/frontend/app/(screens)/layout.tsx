import { ScreenLayout } from '@/widgets/screen-layout';

/** Оболочка всех разделов приложения (карта #556): единый хром новых
 * экранов собирает ScreenLayout — хедер экранов, TabBar/сайдбар, пилюли
 * и PWA-инфраструктура. */
export default function ScreensLayout({ children }: { children: React.ReactNode }) {
  return <ScreenLayout>{children}</ScreenLayout>;
}
