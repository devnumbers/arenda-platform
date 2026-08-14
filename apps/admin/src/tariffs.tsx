import { BooleanField, Datagrid, FunctionField, List, SelectField } from 'react-admin';
import { MoneyField } from './fields';
import { formatPropertyLimit, tariffNameChoices } from './lib/tariffs';

// Экран «Тарифы» — только чтение (issue #247); CRUD приходит отдельным тикетом
// «Админ: CRUD тарифов». Эндпоинт GET /admin/tariffs не принимает sort- и
// пагинационные параметры, поэтому все колонки sortable={false}, пагинация
// выключена (dataProvider не отправляет limit/offset), а порядок определяет
// бэкенд (по цене месяца).
export const TariffList = () => (
  <List pagination={false}>
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
