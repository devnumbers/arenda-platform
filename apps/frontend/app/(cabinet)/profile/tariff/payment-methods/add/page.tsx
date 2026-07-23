'use client';

import { useCallback, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { notify } from '@/shared/lib/notifications';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { Button } from '@/shared/ui/button';
import { useAddPaymentMethod } from '@/features/billing/api/hooks';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import styles from './page.module.css';

export default function AddPaymentMethodPage(): JSX.Element {
  const router = useRouter();
  const add = useAddPaymentMethod();

  const startAdd = useCallback(() => {
    add.mutate(
      {},
      {
        onSuccess: (data) => {
          if (data.confirmUrl) {
            window.location.assign(data.confirmUrl);
            return;
          }

          goBack(router, ROUTES.profilePaymentMethods);
        },
        onError: (error) => {
          notify.scenarios.auth.addCardStartError({description: error.detail});
        },
      },
    );
  }, [add, router]);

  return (
    <PageShell>
      <PageHeader title="Добавить карту" backHref={ROUTES.profilePaymentMethods} />
      <section className={styles.section}>
        {add.isError ? (
          <div className={styles.error}>
            <p className={styles.errorText}>
              Не удалось подключить банковскую форму
            </p>
            <Button
              onClick={startAdd}
              variant="secondary"
              loading={add.isPending}
            >
              Повторить
            </Button>
          </div>
        ) : add.isPending ? (
          <Button loading fullWidth size="large" variant="primary">
            Подключаем банковскую форму…
          </Button>
        ) : (
          <Button
            onClick={startAdd}
            fullWidth
            size="large"
            variant="primary"
          >
            Подключить банковскую форму
          </Button>
        )}
      </section>
    </PageShell>
  );
}
