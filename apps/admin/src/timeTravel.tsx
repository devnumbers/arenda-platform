import { useEffect, useState } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { DateField, useDataProvider, useGetList, useNotify, useRecordContext, useRefresh } from 'react-admin';
import type { AdminDataProvider } from './dataProvider';
import { errorMessage } from './lib/error-message';
import {
  acceptanceScenarioButtons,
  isValidTimeShiftHours,
  latestGraceEntryAt,
  maxTimeShiftHours,
  retrySchedule,
  type TransitionTimestamp,
} from './lib/time-travel';
import { useUserSubscription } from './userSubscription';

// Панель управления временем жизненного цикла подписки (issue #666) —
// стендовый риг над admin-API механизма времени (#665). Роуты рига
// существуют только при BILLING_TIME_TRAVEL: разведка GET
// /admin/billing/time-travel отвечает 404 в сборке без рига, и панель не
// рисуется вовсе — в проде вкладка «Подписка» выглядит как раньше.

const retryTimeFormat = new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit',
  month: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
});

/** Разведка рига: true/false после ответа, null пока ответа нет. */
const useTimeTravelAvailable = (): boolean | null => {
  const dataProvider = useDataProvider<AdminDataProvider>();
  const [available, setAvailable] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    dataProvider
      .billingTimeTravelStatus()
      .then(({ data }) => {
        if (!cancelled) {
          setAvailable(data.enabled === true);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setAvailable(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [dataProvider]);

  return available;
};

/** «Грейс до»: конец окна — тот же validUntil, показывается только в grace. */
const GraceUntilField = () => {
  const subscription = useUserSubscription();
  if (subscription.status !== 'grace' || !subscription.validUntil) {
    return null;
  }
  return <DateField source="subscription.validUntil" label="Грейс до" showTime />;
};

/** Расписание ретраев долга от анкера — последнего перехода grace_entered. */
const RetryScheduleFields = () => {
  const record = useRecordContext();
  const subscription = useUserSubscription();
  const { data, isPending } = useGetList<TransitionTimestamp & { id: string }>('subscriptionTransitions', {
    filter: { owner_id: record?.id },
  });
  const anchor = latestGraceEntryAt(data ?? []);
  // Окна ретраев существуют только в грейсе: вне его фаза долга спит, и
  // расписание от старого анкера было бы ложью. Точная очередность списаний
  // (окна поглощаются платежами с анкера) — предикат бэка, панель показывает
  // только календарь от анкера.
  if (subscription.status !== 'grace' || isPending) {
    return null;
  }
  if (anchor === null) {
    return (
      <Typography variant="body2" color="text.secondary">
        Ретраи долга: анкер не найден в истории переходов.
      </Typography>
    );
  }
  const schedule = retrySchedule(anchor, new Date());
  return (
    <Stack spacing={0.5}>
      <Typography variant="body2">
        Анкер ретраев (вход в grace): {retryTimeFormat.format(new Date(anchor))}
      </Typography>
      {schedule.map((point) => (
        <Typography key={point.label} variant="body2">
          {point.label}: {retryTimeFormat.format(point.at)} — {point.due ? 'окно наступило' : 'ещё рано'}
        </Typography>
      ))}
    </Stack>
  );
};

/** Операции рига: пресеты приёмки, сырой сдвиг и ручной тик фаз воркера. */
const TimeTravelRig = () => {
  const record = useRecordContext();
  const dataProvider = useDataProvider<AdminDataProvider>();
  const notify = useNotify();
  const refresh = useRefresh();
  const [busy, setBusy] = useState(false);
  const [days, setDays] = useState('');
  const [hours, setHours] = useState('');

  const userId = String(record?.id ?? '');
  const totalHours = (Number.parseInt(days, 10) || 0) * 24 + (Number.parseInt(hours, 10) || 0);
  const shiftValid = isValidTimeShiftHours(totalHours);

  const run = async (operation: () => Promise<unknown>, successMessage: string) => {
    setBusy(true);
    try {
      await operation();
      notify(successMessage, { type: 'success' });
      refresh();
    } catch (error) {
      notify(errorMessage(error), { type: 'error' });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Box sx={{ mb: 3 }}>
      <Typography variant="subtitle1" gutterBottom>
        Управление временем (стенд)
      </Typography>
      <Alert severity="info" sx={{ mb: 2 }}>
        Операции когерентно двигают границы подписки (valid_until, анкер ретраев, платежи) — фазы воркера
        догоняют сдвинутое на тике. Каждая операция пишется в журнал переходов ниже и в аудит.
      </Alert>
      <GraceUntilField />
      <Box sx={{ mb: 2 }} />
      <RetryScheduleFields />
      <Box sx={{ mb: 2 }} />
      <Stack direction="row" spacing={1} flexWrap="wrap" sx={{ mb: 2 }}>
        {acceptanceScenarioButtons.map((button) => (
          <Button
            key={button.id}
            variant="outlined"
            size="small"
            disabled={busy}
            onClick={() =>
              void run(
                () => dataProvider.shiftSubscriptionTime({ userId, preset: button.preset }),
                `Пресет «${button.name}» применён`,
              )
            }
          >
            {button.name}
          </Button>
        ))}
      </Stack>
      <Stack direction="row" spacing={1} alignItems="flex-start" flexWrap="wrap" useFlexGap sx={{ mb: 1 }}>
        <TextField
          label="Дней"
          type="number"
          size="small"
          value={days}
          onChange={(event) => setDays(event.target.value)}
          disabled={busy}
          sx={{ width: 110 }}
          slotProps={{ htmlInput: { step: 1 } }}
        />
        <TextField
          label="Часов"
          type="number"
          size="small"
          value={hours}
          onChange={(event) => setHours(event.target.value)}
          disabled={busy}
          sx={{ width: 110 }}
          slotProps={{ htmlInput: { step: 1 } }}
        />
        <Button
          variant="outlined"
          size="small"
          disabled={busy || !shiftValid}
          sx={{ mt: 1 }}
          onClick={() =>
            void run(
              () => dataProvider.shiftSubscriptionTime({ userId, shiftHours: totalHours }),
              `Границы подписки сдвинуты на ${totalHours} ч`,
            )
          }
        >
          Сдвинуть
        </Button>
      </Stack>
      <Typography variant="caption" display="block" sx={{ mb: 2 }}>
        Подписанный сдвиг в часах (можно со знаком минус), один сдвиг ограничен ±{maxTimeShiftHours} ч (±90 дней).
      </Typography>
      <Button
        variant="contained"
        size="small"
        disabled={busy}
        onClick={() => void run(() => dataProvider.billingTick(), 'Фазы воркера прогнаны')}
      >
        Прогнать фазы сейчас
      </Button>
    </Box>
  );
};

/** Панель рига на вкладке «Подписка»: рисуется только там, где риг включён. */
export const TimeTravelPanel = () => {
  const available = useTimeTravelAvailable();
  if (available === null || !available) {
    return null;
  }
  return <TimeTravelRig />;
};
