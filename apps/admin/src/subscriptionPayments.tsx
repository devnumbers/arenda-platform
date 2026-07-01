import { useState } from 'react';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import TextField from '@mui/material/TextField';
import {
  Datagrid,
  DateField,
  FilterButton,
  List,
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

const filters = [
  <TextInput key="user_id" source="user_id" label="ID пользователя" alwaysOn />,
  <TextInput key="status" source="status" label="Статус" alwaysOn />,
];

export const SubscriptionPaymentList = () => (
  <List
    filters={filters}
    actions={
      <TopToolbar>
        <FilterButton />
      </TopToolbar>
    }
  >
    <Datagrid rowClick="show">
      <RaTextField source="id" />
      <RaTextField source="userId" />
      <RaTextField source="status" />
      <RaTextField source="amountKopecks" />
      <RaTextField source="period" />
      <DateField source="createdAt" />
    </Datagrid>
  </List>
);

const RefundButton = () => {
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const record = useRecordContext();
  const [open, setOpen] = useState(false);
  const [amount, setAmount] = useState('');
  const [loading, setLoading] = useState(false);

  if (!record) {
    return null;
  }

  const handleRefund = async () => {
    const trimmed = amount.trim();
    const amountKopecks = trimmed ? Number(trimmed) : undefined;

    if (amountKopecks !== undefined && (!Number.isFinite(amountKopecks) || !Number.isInteger(amountKopecks) || amountKopecks <= 0)) {
      notify('Введите целую положительную сумму возврата или оставьте поле пустым для полного возврата', { type: 'warning' });
      return;
    }

    setLoading(true);
    try {
      await dataProvider.refundPayment({ id: record.id, amountKopecks });
      notify('Возврат выполнен', { type: 'success' });
      setOpen(false);
      setAmount('');
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
          <TextField
            autoFocus
            margin="dense"
            label="Сумма возврата (коп.)"
            type="number"
            fullWidth
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
            helperText="Оставьте пустым для полного возврата"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)} disabled={loading}>
            Отмена
          </Button>
          <Button onClick={handleRefund} disabled={loading}>
            Вернуть
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
      <RaTextField source="id" />
      <RaTextField source="userId" />
      <RaTextField source="status" />
      <RaTextField source="amountKopecks" />
      <RaTextField source="refundedAmountKopecks" />
      <DateField source="createdAt" />
      <DateField source="updatedAt" />
      <DateField source="succeededAt" />
      <RaTextField source="paymentMethodId" />
      <RaTextField source="tariff.name" />
      <RaTextField source="period" />
    </SimpleShowLayout>
  </Show>
);
