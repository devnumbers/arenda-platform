import { BooleanField, Datagrid, FunctionField, List, SelectField, type RaRecord } from 'react-admin';
import { MoneyField } from './fields';
import { formatPropertyLimit, tariffName, tariffNameChoices } from './lib/tariffs';

// Экран «Тарифы» — только чтение (issue #247); CRUD приходит отдельным тикетом
// «Админ: CRUD тарифов». Эндпоинт GET /admin/tariffs не принимает sort-параметры,
// поэтому все колонки sortable={false} и начальная сортировка не задаётся —
// порядок определяет бэкенд (по цене месяца).
export const TariffList = () => (
  <List pagination={false} perPage={100}>
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
    </Datagrid>
  </List>
);

/** Представление записи тарифа: русское название, иначе #id. */
export const tariffRepresentation = (record: RaRecord): string => {
  const name = typeof record.name === 'string' && record.name !== '' ? tariffName(record.name) : '';
  return name || `#${record.id}`;
};
