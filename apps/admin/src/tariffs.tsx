import { useState } from 'react';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
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
  Datagrid,
  FunctionField,
  List,
  SelectField,
  type RaRecord,
  useDataProvider,
  useNotify,
  useRecordContext,
  useRefresh,
} from 'react-admin';
import { MoneyField } from './fields';
import { errorMessage } from './lib/error-message';
import {
  formatPropertyLimit,
  parseTariffNumber,
  tariffName,
  tariffNameChoices,
  validateTariffPricing,
  type TariffName,
} from './lib/tariffs';

// Экран «Тарифы»: чтение (issue #247) + управление (issue #256) — создание,
// изменение цен/лимита и скрытие с подтверждением. Все операции через
// dataProvider (POST/PUT /admin/tariffs), каждая пишет запись в аудит с
// актёром-админом. Эндпоинт GET /admin/tariffs не принимает sort- и
// пагинационные параметры, поэтому колонки sortable={false}, пагинация
// выключена, порядок определяет бэкенд (по цене месяца).

/** Тариф из GET /admin/tariffs. */
interface TariffRecord {
  id?: string;
  name?: string;
  isActive?: boolean;
  activePropertyLimit?: number;
  monthlyPriceKopecks?: number;
  yearlyPriceKopecks?: number;
}

/** Состояние числовых полей диалога: строки, пока пользователь печатает. */
interface PricingDraft {
  monthly: string;
  yearly: string;
  limit: string;
}

const draftOf = (tariff: TariffRecord): PricingDraft => ({
  monthly: String(tariff.monthlyPriceKopecks ?? 0),
  yearly: String(tariff.yearlyPriceKopecks ?? 0),
  limit: String(tariff.activePropertyLimit ?? 0),
});

/** Разбор черновика в числа; null, если какое-то поле не целое число. */
const draftValues = (draft: PricingDraft): { monthly: number; yearly: number; limit: number } | null => {
  const monthly = parseTariffNumber(draft.monthly);
  const yearly = parseTariffNumber(draft.yearly);
  const limit = parseTariffNumber(draft.limit);
  if (monthly === null || yearly === null || limit === null) {
    return null;
  }
  return { monthly, yearly, limit };
};

/** Ошибка черновика (незаполненные поля или нарушение инвариантов) или null. */
const pricingError = (draft: PricingDraft): string | null => {
  const values = draftValues(draft);
  if (!values) {
    return 'Заполните все поля целыми числами';
  }
  return validateTariffPricing({
    monthlyPriceKopecks: values.monthly,
    yearlyPriceKopecks: values.yearly,
    activePropertyLimit: values.limit,
  });
};

/** Поля цен и лимита, общие для диалогов создания и редактирования. */
const PricingFields = ({ draft, onChange }: { draft: PricingDraft; onChange: (draft: PricingDraft) => void }) => (
  <>
    <TextField
      type="number"
      label="Цена за месяц, копейки"
      value={draft.monthly}
      onChange={(event) => onChange({ ...draft, monthly: event.target.value })}
      fullWidth
      slotProps={{ htmlInput: { min: 0 } }}
      sx={{ mt: 1 }}
    />
    <TextField
      type="number"
      label="Цена за год, копейки"
      value={draft.yearly}
      onChange={(event) => onChange({ ...draft, yearly: event.target.value })}
      fullWidth
      slotProps={{ htmlInput: { min: 0 } }}
      sx={{ mt: 2 }}
    />
    <TextField
      type="number"
      label="Лимит активных объектов (−1 — безлимит)"
      value={draft.limit}
      onChange={(event) => onChange({ ...draft, limit: event.target.value })}
      fullWidth
      slotProps={{ htmlInput: { min: -1 } }}
      sx={{ mt: 2 }}
    />
  </>
);

/** Изменение цен и лимита тарифа: PUT /admin/tariffs/{id}. */
const EditTariffButton = () => {
  const record = useRecordContext<TariffRecord>();
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [draft, setDraft] = useState<PricingDraft>({ monthly: '0', yearly: '0', limit: '0' });

  const values = draftValues(draft);
  const validationError = pricingError(draft);

  const handleSubmit = async () => {
    if (!values || !record?.id) {
      return;
    }
    setLoading(true);
    try {
      await dataProvider.update('tariffs', {
        id: record.id,
        previousData: record as RaRecord,
        data: {
          activePropertyLimit: values.limit,
          monthlyPriceKopecks: values.monthly,
          yearlyPriceKopecks: values.yearly,
          isActive: record.isActive !== false,
        },
      });
      notify('Тариф обновлён', { type: 'success' });
      setOpen(false);
      refresh();
    } catch (error) {
      notify(errorMessage(error), { type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button
        size="small"
        onClick={() => {
          setDraft(draftOf(record ?? {}));
          setOpen(true);
        }}
      >
        Изменить
      </Button>
      <Dialog open={open} onClose={() => setOpen(false)} fullWidth>
        <DialogTitle>Изменение тарифа «{tariffName(String(record?.name ?? ''))}»</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{ mb: 1 }}>
            Оплаченные периоды не пересчитываются; следующее продление спишет новую цену.
          </DialogContentText>
          {validationError ? <Alert severity="warning">{validationError}</Alert> : null}
          <PricingFields draft={draft} onChange={setDraft} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)} disabled={loading}>
            Отмена
          </Button>
          <Button onClick={handleSubmit} disabled={loading || validationError !== null} color="primary">
            Сохранить
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
};

/** Скрытие (с явным подтверждением) и обратное включение тарифа. */
const ToggleTariffActivityButton = () => {
  const record = useRecordContext<TariffRecord>();
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const active = record?.isActive !== false;

  const apply = async (isActive: boolean) => {
    if (!record?.id) {
      return;
    }
    setLoading(true);
    try {
      await dataProvider.update('tariffs', {
        id: record.id,
        previousData: record as RaRecord,
        data: { isActive },
      });
      notify(isActive ? 'Тариф снова доступен пользователям' : 'Тариф скрыт', { type: 'success' });
      setConfirmOpen(false);
      refresh();
    } catch (error) {
      notify(errorMessage(error), { type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  if (active) {
    return (
      <>
        <Button size="small" color="warning" onClick={() => setConfirmOpen(true)}>
          Скрыть
        </Button>
        <Dialog open={confirmOpen} onClose={() => setConfirmOpen(false)}>
          <DialogTitle>Скрыть тариф «{tariffName(String(record?.name ?? ''))}»?</DialogTitle>
          <DialogContent>
            <DialogContentText>
              Тариф исчезнет из списка у пользователей и станет недоступен для выбора. Существующие подписки продолжат
              действовать и продлеваться по актуальной цене, история платежей сохранится.
            </DialogContentText>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setConfirmOpen(false)} disabled={loading}>
              Отмена
            </Button>
            <Button onClick={() => apply(false)} disabled={loading} color="warning">
              Скрыть тариф
            </Button>
          </DialogActions>
        </Dialog>
      </>
    );
  }

  return (
    <Button size="small" disabled={loading} onClick={() => apply(true)}>
      Показать
    </Button>
  );
};

/** Действия над тарифом в строке списка. */
const TariffRowActions = () => (
  <Stack direction="row" spacing={1} alignItems="center">
    <EditTariffButton />
    <ToggleTariffActivityButton />
  </Stack>
);

/** Создание тарифа: POST /admin/tariffs. */
export const CreateTariffButton = () => {
  const dataProvider = useDataProvider();
  const notify = useNotify();
  const refresh = useRefresh();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [name, setName] = useState<TariffName>('basic');
  const [draft, setDraft] = useState<PricingDraft>({ monthly: '0', yearly: '0', limit: '1' });

  const values = draftValues(draft);
  const validationError = pricingError(draft);

  const handleSubmit = async () => {
    if (!values) {
      return;
    }
    setLoading(true);
    try {
      await dataProvider.create('tariffs', {
        data: {
          name,
          activePropertyLimit: values.limit,
          monthlyPriceKopecks: values.monthly,
          yearlyPriceKopecks: values.yearly,
        },
      });
      notify('Тариф создан', { type: 'success' });
      setOpen(false);
      refresh();
    } catch (error) {
      notify(errorMessage(error), { type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button variant="contained" onClick={() => setOpen(true)}>
        Создать тариф
      </Button>
      <Dialog open={open} onClose={() => setOpen(false)} fullWidth>
        <DialogTitle>Новый тариф</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{ mb: 1 }}>
            Название — из фиксированного набора пользовательского контракта; занятое имя бэкенд отклонит (409).
          </DialogContentText>
          <TextField
            select
            label="Название"
            value={name}
            onChange={(event) => setName(event.target.value as TariffName)}
            fullWidth
          >
            {tariffNameChoices.map((choice) => (
              <MenuItem key={choice.id} value={choice.id}>
                {choice.name}
              </MenuItem>
            ))}
          </TextField>
          {validationError ? <Alert severity="warning">{validationError}</Alert> : null}
          <PricingFields draft={draft} onChange={setDraft} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpen(false)} disabled={loading}>
            Отмена
          </Button>
          <Button onClick={handleSubmit} disabled={loading || validationError !== null} color="primary">
            Создать
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
};

/** Тулбар списка: только создание тарифа (issue #256). */
const TariffListActions = () => <CreateTariffButton />;

export const TariffList = () => (
  <List pagination={false} actions={<TariffListActions />}>
    <Datagrid bulkActionButtons={false}>
      <SelectField source="name" choices={tariffNameChoices} sortable={false} />
      <BooleanField source="isActive" valueLabelTrue="Активен" valueLabelFalse="Скрыт" sortable={false} />
      <FunctionField
        source="activePropertyLimit"
        render={(record) => formatPropertyLimit(record.activePropertyLimit)}
        sortable={false}
      />
      <MoneyField source="monthlyPriceKopecks" sortable={false} />
      <MoneyField source="yearlyPriceKopecks" sortable={false} />
      <FunctionField label="Действия" render={() => <TariffRowActions />} sortable={false} />
    </Datagrid>
  </List>
);
