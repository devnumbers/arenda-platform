import { useState } from 'react';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import {
  Datagrid,
  DateField,
  FilterButton,
  List,
  SelectField,
  SelectInput,
  Show,
  SimpleShowLayout,
  TextField as RaTextField,
  TextInput,
  TopToolbar,
  useDataProvider,
  useNotify,
  useRecordContext,
  useRefresh,
} from 'react-admin';
import {
  ChoiceChipField,
  MoneyField,
  UserLinkField,
  UserReferenceField,
  subscriptionPaymentPeriodChoices,
  subscriptionPaymentStatusChoices,
  subscriptionStatusChoices,
} from './fields';

const filters = [
  <TextInput key="user_phone" source="user_phone" label="Телефон пользователя" alwaysOn />,
  <SelectInput key="status" source="status" label="Статус платежа" choices={subscriptionPaymentStatusChoices} alwaysOn />,
  <SelectInput
    key="subscription_status"
    source="subscription_status"
    label="Статус подписки"
    choices={subscriptionStatusChoices}
    alwaysOn
  />,
];

// sortable={false} проставлен колонкам вне whitelist сортировки бэкенда
// (payments: createdAt, amountKopecks, status) — иначе бэкенд отвечает 400.
export const SubscriptionPaymentList = () => (
  <List
    filters={filters}
    sort={{ field: 'createdAt', order: 'DESC' }}
    actions={
      <TopToolbar>
        <FilterButton />
      </TopToolbar>
    }
  >
    <Datagrid rowClick="show" bulkActionButtons={false}>
      <UserLinkField source="userId" sortable={false} />
      <ChoiceChipField source="status" choices={subscriptionPaymentStatusChoices} />
      <MoneyField source="amountKopecks" />
      <SelectField source="period" choices={subscriptionPaymentPeriodChoices} sortable={false} />
      <DateField source="createdAt" showTime />
    </Datagrid>
  </List>
);

// Возвраты всегда полные (ADR 0037): контракт эндпоинта не принимает сумму,
// диалог — только подтверждение действия.
const RefundButton = () => {
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const record = useRecordContext();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  if (!record) {
    return null;
  }

  const handleRefund = async () => {
    setLoading(true);
    try {
      await dataProvider.refundPayment({ id: record.id });
      notify('Возврат выполнен', { type: 'success' });
      setOpen(false);
      refresh();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Ошибка возврата';
      notify(message, { type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button variant="outlined" color="error" onClick={() => setOpen(true)}>
        Вернуть платёж
      </Button>
      <Dialog open={open} onClose={() => setOpen(false)}>
        <DialogTitle>Возврат платежа</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Платёж будет возвращён полностью. Подписка пользователя перейдёт на базовый тариф,
            избыточные объекты будут архивированы.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)} disabled={loading}>
            Отмена
          </Button>
          <Button onClick={handleRefund} disabled={loading} color="error">
            Вернуть полностью
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
};

const SyncButton = () => {
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const record = useRecordContext();

  if (!record) {
    return null;
  }

  const handleSync = async () => {
    try {
      await dataProvider.syncPayment({ id: record.id });
      notify('Платёж синхронизирован', { type: 'success' });
      refresh();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Ошибка синхронизации';
      notify(message, { type: 'error' });
    }
  };

  return (
    <Button variant="outlined" onClick={handleSync}>
      Синхронизировать
    </Button>
  );
};

const SubscriptionPaymentActions = () => (
  <TopToolbar>
    <RefundButton />
    <SyncButton />
  </TopToolbar>
);

export const SubscriptionPaymentShow = () => (
  <Show actions={<SubscriptionPaymentActions />}>
    <SimpleShowLayout>
      <UserReferenceField source="userId" />
      <SelectField source="status" choices={subscriptionPaymentStatusChoices} />
      <MoneyField source="amountKopecks" />
      <MoneyField source="refundedAmountKopecks" />
      <RaTextField source="tariff.name" />
      <SelectField source="period" choices={subscriptionPaymentPeriodChoices} />
      <DateField source="succeededAt" showTime />
      <DateField source="createdAt" showTime />
      <DateField source="updatedAt" showTime />
      <RaTextField source="paymentMethodId" />
      <RaTextField source="id" />
    </SimpleShowLayout>
  </Show>
);
