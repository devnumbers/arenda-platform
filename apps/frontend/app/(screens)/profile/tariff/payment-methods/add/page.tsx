'use client';

import { useCallback, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  PageContent,
  TopNav,
  TopNavBackButton,
  TopNavTitle,
} from '@/shared/ui/design';
import { Button } from '@/shared/ui/button';
import { useAddPaymentMethod } from '@/features/billing';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import styles from './page.module.css';

export default function AddPaymentMethodPage(): JSX.Element {
  const router = useRouter();
  const add = useAddPaymentMethod();

  // Стабильная идентичность обработчика: уходит в мутацию и три кнопки
  // (ошибка/ожидание/приглашение) — без него у кнопок разные экземпляры
  // и loading-состояния разъезжаются.
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
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profilePaymentMethods} />}>
        <TopNavTitle title="Добавить карту" />
      </TopNav>
      <PageContent className="px-6">
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
      </PageContent>
    </>
  );
}
