import { redirect } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';

// /dashboard был главной кабинета с арендами и операциями; после удаления
// домена (спека #434) дом кабинета — /properties. Роут остаётся как
// постоянный редирект: он зашит в start_url манифеста PWA (менять start_url
// после публикации нельзя) и в старых ссылках/закладках пользователей.
export default function DashboardRedirectPage(): never {
  redirect(ROUTES.properties);
}
