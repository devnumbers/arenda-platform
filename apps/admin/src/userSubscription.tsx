import { useState, type ReactNode } from 'react';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import {
  BooleanField,
  DateField,
  Datagrid,
  FunctionField,
  ReferenceManyField,
  SelectField,
  TextField as RaTextField,
  useDataProvider,
  useGetList,
  useNotify,
  useRecordContext,
  useRefresh,
} from 'react-admin';
import { ChoiceChipField, subscriptionStatusChoices } from './fields';
import {
  canCancelOnBehalf,
  canExtendGrace,
  paidRemainderWarning,
  subscriptionSourceChoices,
  transitionInitiatorChoices,
  transitionReasonLabel,
  type UserSubscriptionRecord,
} from './lib/subscription';

// Блок «Подписка» карточки пользователя (issue #255): текущее состояние,
// кнопки админ-операций и история переходов. Операции идут через
// dataProvider, каждая пишет запись в историю переходов и аудит с актёром-админом.

/** Подписка карточки пользователя: record.subscription из GET /admin/users/{id}. */
const useUserSubscription = (): UserSubscriptionRecord => {
  const record = useRecordContext();
  const subscription = (record as Record<string, unknown> | undefined)?.subscription;
  return subscription && typeof subscription === 'object' ? (subscription as UserSubscriptionRecord) : {};
};

/** Сообщение об ошибке провайдера для тоста. */
const errorMessage = (error: unknown): string => (error instanceof Error ? error.message : 'Ошибка операции');

/** Ссылка на Show платежа из истории переходов. */
const TransitionPaymentField = () => {
  const record = useRecordContext();
  if (!record?.paymentId) {
    return null;
  }
  return (
    <Button size="small" href={`#/subscriptionPayments/${record.paymentId}/show`}>
      Платёж
    </Button>
  );
};

/** Причина перехода: русский лейбл из словаря, неизвестная — как есть. */
const TransitionReasonField = () => {
  const record = useRecordContext();
  if (!record) {
    return null;
  }
  return <span>{transitionReasonLabel(String(record.reason))}</span>;
};

/** Смена статуса/тарифа в одной ячейке: «было → стало», «—» для первой записи. */
const TransitionChangeField = ({ from, to }: { from: string; to: string }) => {
  const record = useRecordContext() as Record<string, unknown> | undefined;
  if (!record) {
    return null;
  }
  const fromValue = record[from];
  const toValue = record[to];
  return <span>{fromValue ? `${String(fromValue)} → ${String(toValue)}` : String(toValue)}</span>;
};

/** История переходов подписки пользователя, новые сверху. */
export const SubscriptionTransitionsPanel = () => (
  <ReferenceManyField reference="subscriptionTransitions" target="owner_id" label={false}>
    <Datagrid bulkActionButtons={false}>
      <DateField source="createdAt" showTime sortable={false} />
      <FunctionField label="Причина" render={() => <TransitionReasonField />} sortable={false} />
      <ChoiceChipField source="initiator" choices={transitionInitiatorChoices} sortable={false} />
      <FunctionField label="Статус" render={() => <TransitionChangeField from="fromStatus" to="toStatus" />} sortable={false} />
      <FunctionField label="Тариф" render={() => <TransitionChangeField from="fromTariffName" to="toTariffName" />} sortable={false} />
      <FunctionField label="Платёж" render={() => <TransitionPaymentField />} sortable={false} />
    </Datagrid>
  </ReferenceManyField>
);

// Диалог операции: кнопка + форма из контролируемых MUI-полей, тем же
// минимальным набором, что и RefundButton в subscriptionPayments.tsx.
const OperationDialog = ({
  label,
  title,
  description,
  open,
  onOpen,
  onClose,
  onSubmit,
  submitLabel,
  loading,
  canSubmit,
  children,
}: {
  label: string;
  title: string;
  description?: ReactNode;
  open: boolean;
  onOpen: () => void;
  onClose: () => void;
  onSubmit: () => void;
  submitLabel: string;
  loading: boolean;
  canSubmit?: boolean;
  children?: ReactNode;
}) => (
  <>
    <Button variant="outlined" onClick={onOpen}>
      {label}
    </Button>
    <Dialog open={open} onClose={onClose} fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        {description ? <DialogContentText sx={{ mb: 2 }}>{description}</DialogContentText> : null}
        {children}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          Отмена
        </Button>
        <Button onClick={onSubmit} disabled={loading || !canSubmit} color="primary">
          {submitLabel}
        </Button>
      </DialogActions>
    </Dialog>
  </>
);

interface TariffOption {
  id: string;
  name: string;
}

/** Select тарифа из GET /admin/tariffs (все планы, включая скрытые). */
const TariffSelect = ({ value, onChange }: { value: string; onChange: (name: string) => void }) => {
  const { data, isPending } = useGetList<TariffOption>('tariffs', {
    pagination: { page: 1, perPage: 100 },
  });
  return (
    <TextField select label="Тариф" value={value} onChange={(event) => onChange(event.target.value)} fullWidth disabled={isPending}>
      {(data ?? []).map((tariff) => (
        <MenuItem key={tariff.id} value={tariff.name}>
          {tariff.name}
        </MenuItem>
      ))}
    </TextField>
  );
};

/** Назначение служебной подписки: тариф на фиксированный срок без оплаты. */
const AssignServiceButton = () => {
  const record = useRecordContext();
  const subscription = useUserSubscription();
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [tariffName, setTariffName] = useState('');
  const [termType, setTermType] = useState<'month' | 'year' | 'date'>('month');
  const [untilDate, setUntilDate] = useState('');

  const warning = open ? paidRemainderWarning(subscription, new Date()) : null;

  const handleSubmit = async () => {
    setLoading(true);
    try {
      await dataProvider.assignServiceSubscription({
        userId: String(record?.id ?? ''),
        tariffName,
        termType,
        untilDate: termType === 'date' ? untilDate : undefined,
      });
      notify('Служебная подписка назначена', { type: 'success' });
      setOpen(false);
      refresh();
    } catch (error) {
      notify(errorMessage(error), { type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <OperationDialog
      label="Назначить служебную"
      title="Назначить служебную подписку"
      description="Тариф на фиксированный срок без оплаты, автопродление выключено. По истечении срока подписка перейдёт на базовый тариф, избыточные объекты будут архивированы."
      open={open}
      onOpen={() => {
        setTariffName('');
        setTermType('month');
        setUntilDate('');
        setOpen(true);
      }}
      onClose={() => setOpen(false)}
      onSubmit={handleSubmit}
      submitLabel="Назначить"
      loading={loading}
      canSubmit={tariffName !== '' && (termType !== 'date' || untilDate !== '')}
    >
      {warning ? <Alert severity="warning" sx={{ mb: 1 }}>{warning}</Alert> : null}
      <TariffSelect value={tariffName} onChange={setTariffName} />
      <TextField
        select
        label="Срок"
        value={termType}
        onChange={(event) => setTermType(event.target.value as 'month' | 'year' | 'date')}
        fullWidth
        sx={{ mt: 2 }}
      >
        <MenuItem value="month">Месяц</MenuItem>
        <MenuItem value="year">Год</MenuItem>
        <MenuItem value="date">До даты</MenuItem>
      </TextField>
      {termType === 'date' ? (
        <TextField
          type="date"
          label="Действует до (включительно)"
          value={untilDate}
          onChange={(event) => setUntilDate(event.target.value)}
          fullWidth
          slotProps={{ inputLabel: { shrink: true } }}
          sx={{ mt: 2 }}
        />
      ) : null}
    </OperationDialog>
  );
};

/** Принудительная смена тарифа: мгновенно, без оплаты, с архивацией избытка. */
const ForceChangeTariffButton = () => {
  const record = useRecordContext();
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [tariffName, setTariffName] = useState('');
  const [period, setPeriod] = useState<'month' | 'year'>('month');

  const handleSubmit = async () => {
    setLoading(true);
    try {
      await dataProvider.forceChangeSubscriptionTariff({ userId: String(record?.id ?? ''), tariffName, period });
      notify('Тариф изменён', { type: 'success' });
      setOpen(false);
      refresh();
    } catch (error) {
      notify(errorMessage(error), { type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <OperationDialog
      label="Принудительно сменить тариф"
      title="Принудительная смена тарифа"
      description="Новый тариф применяется мгновенно и без оплаты на выбранный срок от текущего момента. Если лимит активных объектов нового тарифа меньше текущего, избыточные объекты будут архивированы."
      open={open}
      onOpen={() => {
        setTariffName('');
        setPeriod('month');
        setOpen(true);
      }}
      onClose={() => setOpen(false)}
      onSubmit={handleSubmit}
      submitLabel="Сменить тариф"
      loading={loading}
      canSubmit={tariffName !== ''}
    >
      <TariffSelect value={tariffName} onChange={setTariffName} />
      <TextField
        select
        label="Период"
        value={period}
        onChange={(event) => setPeriod(event.target.value as 'month' | 'year')}
        fullWidth
        sx={{ mt: 2 }}
      >
        <MenuItem value="month">Месяц</MenuItem>
        <MenuItem value="year">Год</MenuItem>
      </TextField>
    </OperationDialog>
  );
};

/** Продление грейс-периода вручную: дополнительные дни на починку карты. */
const ExtendGraceButton = () => {
  const record = useRecordContext();
  const subscription = useUserSubscription();
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [days, setDays] = useState('7');

  if (!canExtendGrace(subscription)) {
    return null;
  }

  const handleSubmit = async () => {
    setLoading(true);
    try {
      await dataProvider.extendSubscriptionGrace({ userId: String(record?.id ?? ''), days: Number(days) });
      notify('Грейс-период продлён', { type: 'success' });
      setOpen(false);
      refresh();
    } catch (error) {
      notify(errorMessage(error), { type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <OperationDialog
      label="Продлить грейс"
      title="Продлить грейс-период"
      description="Пользователь получит дополнительные дни на обновление способа оплаты до перехода на базовый тариф."
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      onSubmit={handleSubmit}
      submitLabel="Продлить"
      loading={loading}
    >
      <TextField
        type="number"
        label="Дней (1–90)"
        value={days}
        onChange={(event) => setDays(event.target.value)}
        fullWidth
        slotProps={{ htmlInput: { min: 1, max: 90 } }}
        sx={{ mt: 1 }}
      />
    </OperationDialog>
  );
};

/** Отмена подписки от имени пользователя (по просьбе поддержки). */
const CancelSubscriptionButton = () => {
  const record = useRecordContext();
  const subscription = useUserSubscription();
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  if (!canCancelOnBehalf(subscription)) {
    return null;
  }

  const handleCancel = async () => {
    setLoading(true);
    try {
      await dataProvider.cancelSubscription({ userId: String(record?.id ?? '') });
      notify('Подписка отменена', { type: 'success' });
      setOpen(false);
      refresh();
    } catch (error) {
      notify(errorMessage(error), { type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <OperationDialog
      label="Отменить подписку"
      title="Отмена подписки от имени пользователя"
      description="Автопродление выключится сразу, тариф продолжит работать до конца оплаченного периода. Восстановление — через оплату тарифа самим пользователем."
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      onSubmit={handleCancel}
      submitLabel="Отменить подписку"
      loading={loading}
    />
  );
};

/** Кнопки операций блока «Подписка». */
export const SubscriptionActions = () => (
  <Stack direction="row" spacing={1} flexWrap="wrap" sx={{ mb: 2 }}>
    <AssignServiceButton />
    <ForceChangeTariffButton />
    <ExtendGraceButton />
    <CancelSubscriptionButton />
  </Stack>
);

/** Текущее состояние подписки блока «Подписка». */
export const SubscriptionStateFields = () => {
  const subscription = useUserSubscription();
  if (!subscription.status) {
    return <Alert severity="info">У пользователя нет подписки.</Alert>;
  }
  const sourceChoice = subscriptionSourceChoices.find((choice) => choice.id === subscription.source);
  return (
    <Stack spacing={1} sx={{ mb: 2 }}>
      <Chip
        size="small"
        label={subscription.source ? (sourceChoice?.name ?? String(subscription.source)) : '—'}
        sx={{ alignSelf: 'flex-start' }}
      />
      <SelectField source="subscription.status" choices={subscriptionStatusChoices} />
      <RaTextField source="subscription.tariff.name" label="Тариф" />
      <DateField source="subscription.validUntil" label="Действует до" showTime />
      <BooleanField source="subscription.autoRenewEnabled" label="Автопродление" />
    </Stack>
  );
};
