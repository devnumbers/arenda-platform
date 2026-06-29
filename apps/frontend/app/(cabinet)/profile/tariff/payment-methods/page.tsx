import type { Metadata } from 'next';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { PaymentMethodList } from '@/widgets/profile';
import { ROUTES } from '@/shared/config/routes';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Способы оплаты — Arenda Platform',
  description: 'Управление способами оплаты',
};

type PaymentMethodsPageProps = {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export default async function PaymentMethodsPage({
  searchParams,
}: PaymentMethodsPageProps) {
  const params = await searchParams;
  const addCardResult =
    typeof params.addCard === 'string' ? params.addCard : undefined;

  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <header className={styles.header}>
          <IconLink
            href={ROUTES.profileTariff}
            aria-label="Назад"
            icon={<ArrowLeft />}
          />
          <h1 className={styles.title}>Способы оплаты</h1>
        </header>
        <PaymentMethodList addCardResult={addCardResult} />
      </div>
    </div>
  );
}
