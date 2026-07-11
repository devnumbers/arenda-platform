import { redirect } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';

export default function Home() {
  redirect(ROUTES.dashboard);
}
