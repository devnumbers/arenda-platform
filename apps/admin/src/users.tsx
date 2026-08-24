import {
  Datagrid,
  DateField,
  FilterForm,
  List,
  ReferenceManyField,
  SelectField,
  SelectInput,
  Show,
  Tab,
  TabbedShowLayout,
  TextField,
  TextInput,
} from 'react-admin';
import {
  ChoiceChipField,
  FullNameField,
  auditActionChoices,
  propertyStatusChoices,
  propertyStatusFilterChoices,
  propertyTypeChoices,
  roleChoices,
  subscriptionStatusChoices,
} from './fields';
import { AuditLogDatagrid } from './auditLogs';
import { SubscriptionActions, SubscriptionStateFields, SubscriptionTransitionsPanel } from './userSubscription';

const userFilters = [
  <TextInput key="phone" source="phone" label="Телефон" />,
  <TextInput key="email" source="email" label="Email" />,
  <SelectInput key="role" source="role" label="Роль" choices={roleChoices} />,
  <SelectInput
    key="subscription_status"
    source="subscription_status"
    label="Статус подписки"
    choices={subscriptionStatusChoices}
  />,
];

// Фильтры вложенных табов UserShow: nested-эндпоинт объектов принимает status
// (query-enum — только active/all/archived). alwaysOn обязателен: внутри
// ReferenceManyField нет FilterButton, иначе фильтры скрыты.
const userPropertyTabFilters = [
  <SelectInput key="status" source="status" label="Статус" choices={propertyStatusFilterChoices} alwaysOn />,
];

// Фильтры вкладки «Журнал действий»: nested-эндпоинт принимает
// action/entity_type/date_from/date_to; на вкладке оставлен только action.
const userAuditLogTabFilters = [
  <SelectInput key="action" source="action" label="Действие" choices={auditActionChoices} alwaysOn />,
];

// sortable={false} проставлен колонкам вне whitelist сортировки бэкенда
// (users: createdAt, updatedAt) — иначе бэкенд отвечает 400.
export const UserList = () => (
  <List filters={userFilters} sort={{ field: 'createdAt', order: 'DESC' }}>
    <Datagrid rowClick="show" bulkActionButtons={false}>
      <TextField source="phone" sortable={false} />
      <FullNameField source="surname" label="ФИО" sortable={false} />
      <TextField source="email" sortable={false} />
      <TextField source="role" sortable={false} />
      <ChoiceChipField source="subscriptionStatus" choices={subscriptionStatusChoices} sortable={false} />
      <DateField source="createdAt" showTime />
    </Datagrid>
  </List>
);

export const UserShow = () => (
  <Show>
    <TabbedShowLayout>
      <Tab label="Профиль">
        <TextField source="phone" />
        <TextField source="email" />
        <SelectField source="role" choices={roleChoices} />
        <DateField source="createdAt" showTime />
        <DateField source="updatedAt" showTime />
        <TextField source="id" />
      </Tab>
      <Tab label="Объекты">
        <ReferenceManyField
          reference="properties"
          target="owner_id"
          label={false}
          sort={{ field: 'updatedAt', order: 'DESC' }}
        >
          <FilterForm filters={userPropertyTabFilters} />
          <Datagrid rowClick="show" bulkActionButtons={false}>
            <TextField source="name" />
            <SelectField source="type" choices={propertyTypeChoices} sortable={false} />
            <ChoiceChipField source="status" choices={propertyStatusChoices} />
            <TextField source="address" sortable={false} />
            <DateField source="createdAt" showTime />
          </Datagrid>
        </ReferenceManyField>
      </Tab>
      <Tab label="Журнал действий">
        <ReferenceManyField
          reference="auditLogs"
          target="owner_id"
          label={false}
          sort={{ field: 'createdAt', order: 'DESC' }}
        >
          <FilterForm filters={userAuditLogTabFilters} />
          <AuditLogDatagrid />
        </ReferenceManyField>
      </Tab>
      <Tab label="Подписка">
        <SubscriptionActions />
        <SubscriptionStateFields />
        <SubscriptionTransitionsPanel />
      </Tab>
      <Tab label="Статистика">
        <TextField source="stats.activePropertiesCount" />
        <TextField source="stats.archivedPropertiesCount" />
      </Tab>
    </TabbedShowLayout>
  </Show>
);
