import {
  Datagrid,
  DateField,
  List,
  ReferenceManyField,
  SelectInput,
  Show,
  Tab,
  TabbedShowLayout,
  TextField,
  TextInput,
} from 'react-admin';

const userFilters = [
  <TextInput key="phone" source="phone" label="Телефон" />,
  <TextInput key="email" source="email" label="Email" />,
  <SelectInput
    key="role"
    source="role"
    label="Роль"
    choices={[
      { id: 'owner', name: 'Собственник' },
      { id: 'admin', name: 'Администратор' },
    ]}
  />,
  <SelectInput
    key="subscription_status"
    source="subscription_status"
    label="Статус подписки"
    choices={[
      { id: 'active', name: 'Активна' },
      { id: 'grace', name: 'Грейс-период' },
      { id: 'blocked', name: 'Заблокирована' },
      { id: 'cancelled', name: 'Отменена' },
    ]}
  />,
];

export const UserList = () => (
  <List filters={userFilters}>
    <Datagrid rowClick="show">
      <TextField source="id" />
      <TextField source="phone" />
      <TextField source="email" />
      <TextField source="role" />
      <DateField source="createdAt" />
    </Datagrid>
  </List>
);

export const UserShow = () => (
  <Show>
    <TabbedShowLayout>
      <Tab label="Профиль">
        <TextField source="id" />
        <TextField source="phone" />
        <TextField source="email" />
        <TextField source="role" />
        <DateField source="createdAt" />
        <DateField source="updatedAt" />
      </Tab>
      <Tab label="Объекты">
        <ReferenceManyField reference="properties" target="owner_id" label={false}>
          <Datagrid rowClick="show">
            <TextField source="id" />
            <TextField source="name" />
            <TextField source="type" />
            <TextField source="status" />
            <TextField source="address" />
            <TextField source="occupancy" />
            <DateField source="createdAt" />
          </Datagrid>
        </ReferenceManyField>
      </Tab>
      <Tab label="Договоры">
        <ReferenceManyField reference="leases" target="owner_id" label={false}>
          <Datagrid rowClick="show">
            <TextField source="id" />
            <TextField source="status" />
            <TextField source="rentAmountKopecks" />
            <TextField source="depositAmountKopecks" />
            <TextField source="paymentDay" />
            <DateField source="startDate" />
            <DateField source="endDate" />
          </Datagrid>
        </ReferenceManyField>
      </Tab>
      <Tab label="Контакты">
        <ReferenceManyField reference="tenantContacts" target="owner_id" label={false}>
          <Datagrid rowClick="show">
            <TextField source="id" />
            <TextField source="name" />
            <TextField source="surname" />
            <TextField source="phone" />
            <TextField source="email" />
          </Datagrid>
        </ReferenceManyField>
      </Tab>
      <Tab label="Операции">
        <ReferenceManyField reference="operations" target="owner_id" label={false}>
          <Datagrid rowClick="show">
            <TextField source="id" />
            <TextField source="type" />
            <TextField source="category" />
            <TextField source="name" />
            <TextField source="amountKopecks" />
            <TextField source="status" />
            <DateField source="operationDate" />
          </Datagrid>
        </ReferenceManyField>
      </Tab>
      <Tab label="Подписка">
        <TextField source="subscription.status" />
        <TextField source="subscription.tariff.name" />
        <DateField source="subscription.validUntil" />
        <TextField source="subscription.autoRenewEnabled" />
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
