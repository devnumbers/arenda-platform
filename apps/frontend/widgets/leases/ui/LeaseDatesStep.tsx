'use client';

import type {JSX} from 'react';
import {useMemo} from 'react';
import {parseDate} from '@internationalized/date';
import {Button} from '@/shared/ui/button';
import {Select} from '@/shared/ui/select';
import type {TenantContact} from '@/entities/tenant-contact/model/types';
import {DateSelect} from './DateSelect';
import {PaymentDayPicker} from './PaymentDayPicker';
import styles from './LeaseDatesStep.module.css';

export type LeaseDatesStepProps = {
  readonly paymentDay?: number;
  readonly startDate?: string;
  readonly endDate?: string;
  readonly tenantContactId?: string;
  readonly tenantContacts?: TenantContact[];
  readonly onPaymentDayChange: (day: number) => void;
  readonly onStartDateChange: (date: string) => void;
  readonly onEndDateChange: (date: string) => void;
  readonly onTenantContactChange: (id: string) => void;
  readonly onCreateTenant: () => void;
  readonly onSubmit: () => void;
  readonly isLoading: boolean;
  readonly error?: string;
};

export function LeaseDatesStep({
  paymentDay,
  startDate,
  endDate,
  tenantContactId,
  tenantContacts,
  onPaymentDayChange,
  onStartDateChange,
  onEndDateChange,
  onTenantContactChange,
  onCreateTenant,
  onSubmit,
  isLoading,
  error,
}: LeaseDatesStepProps): JSX.Element {
  const tenantContactOptions = useMemo(
    () =>
      tenantContacts?.map((contact) => ({
        value: contact.id,
        label: [contact.surname, contact.name, contact.patronymic]
          .filter(Boolean)
          .join(' '),
      })) ?? [],
    [tenantContacts]
  );

  const handleStartDateChange = (date: string | undefined) => {
    if (!date) return;
    onStartDateChange(date);
  };

  const handleEndDateChange = (date: string | undefined) => {
    onEndDateChange(date ?? '');
  };

  const startDateValid = startDate !== undefined && startDate !== '';
  const datesOrdered =
    !startDateValid ||
    endDate === undefined ||
    endDate === '' ||
    parseDate(startDate).compare(parseDate(endDate)) <= 0;

  const isValid =
    paymentDay !== undefined &&
    paymentDay >= 1 &&
    paymentDay <= 31 &&
    startDateValid &&
    datesOrdered;

  return (
    <div className={styles.root}>
      <h2 className={styles.heading}>Даты аренды</h2>
      <div className={styles.fields}>
        <PaymentDayPicker value={paymentDay} onChange={onPaymentDayChange}/>
        <div className={styles.dateRow}>
          <DateSelect
            label="Начало аренды"
            value={startDate}
            onChange={handleStartDateChange}
            required
          />
          <DateSelect
            label="Конец аренды"
            value={endDate}
            minValue={startDate}
            onChange={handleEndDateChange}
          />
        </div>

        <div className={styles.tenantField}>
          <Select
            label="Арендатор"
            value={tenantContactId ?? ''}
            options={tenantContactOptions}
            onChange={(id) => onTenantContactChange(id)}
            placeholder="Не указан"
          />
          <button
            type="button"
            className={styles.createTenantLink}
            onClick={onCreateTenant}
          >
            Создать нового арендатора
          </button>
        </div>
      </div>
      <div className={styles.submit}>
        {error && <p className={styles.error}>{error}</p>}
        <Button
          type="button"
          variant="primary"
          size="large"
          fullWidth
          disabled={!isValid}
          loading={isLoading}
          onClick={onSubmit}
        >
          Создать аренду
        </Button>
      </div>
    </div>
  );
}
