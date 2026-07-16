import {
  BooleanField,
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
  occupancyChoices,
  operationStatusChoices,
  operationTypeChoices,
  propertyStatusChoices,
  propertyStatusFilterChoices,
  propertyTypeChoices,
  roleChoices,
  subscriptionStatusChoices,
} from './fields';
import { LeaseDatagrid } from './leases';
import { OperationDatagrid } from './operations';

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

// Фильтры вложенных табов UserShow: nested-эндпоинты принимают status (properties)
// и status/type (operations); query-enum статуса объектов — только active/all/archived.
// alwaysOn обязателен: внутри ReferenceManyField нет FilterButton, иначе фильтры скрыты.
const userPropertyTabFilters = [
  <SelectInput key="status" source="status" label="Статус" choices={propertyStatusFilterChoices} alwaysOn />,
];

const userOperationTabFilters = [
  <SelectInput key="status" source="status" label="Статус" choices={operationStatusChoices} alwaysOn />,
  <SelectInput key="type" source="type" label="Тип" choices={operationTypeChoices} alwaysOn />,
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
            <SelectField source="occupancy" choices={occupancyChoices} sortable={false} />
            <DateField source="createdAt" showTime />
          </Datagrid>
        </ReferenceManyField>
      </Tab>
      <Tab label="Договоры">
        <ReferenceManyField
          reference="leases"
          target="owner_id"
          label={false}
          sort={{ field: 'updatedAt', order: 'DESC' }}
        >
          <LeaseDatagrid />
        </ReferenceManyField>
      </Tab>
      <Tab label="Контакты">
        <ReferenceManyField
          reference="tenantContacts"
          target="owner_id"
          label={false}
          sort={{ field: 'name', order: 'ASC' }}
        >
          <Datagrid rowClick="show" bulkActionButtons={false}>
            <TextField source="name" />
            <TextField source="surname" sortable={false} />
            <TextField source="phone" sortable={false} />
            <TextField source="email" sortable={false} />
          </Datagrid>
        </ReferenceManyField>
      </Tab>
      <Tab label="Операции">
        <ReferenceManyField
          reference="operations"
          target="owner_id"
          label={false}
          sort={{ field: 'operationDate', order: 'DESC' }}
        >
          <FilterForm filters={userOperationTabFilters} />
          <OperationDatagrid />
        </ReferenceManyField>
      </Tab>
      <Tab label="Подписка">
        <SelectField source="subscription.status" choices={subscriptionStatusChoices} />
        <TextField source="subscription.tariff.name" />
        <DateField source="subscription.validUntil" />
        <BooleanField source="subscription.autoRenewEnabled" />
      </Tab>
      <Tab label="Статистика">
        <TextField source="stats.activePropertiesCount" />
        <TextField source="stats.archivedPropertiesCount" />
        <TextField source="stats.leasesCount" />
        <TextField source="stats.tenantContactsCount" />
        <TextField source="stats.operationsCount" />
      </Tab>
    </TabbedShowLayout>
  </Show>
);
