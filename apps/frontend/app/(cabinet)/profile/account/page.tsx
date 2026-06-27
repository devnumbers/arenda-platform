import type { Metadata } from 'next';
import { AccountOverview } from '@/widgets/profile';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Аккаунт — Arenda Platform',
  description: 'Управление аккаунтом пользователя',
};

export default function AccountPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <AccountOverview />
      </div>
    </div>
  );
}
