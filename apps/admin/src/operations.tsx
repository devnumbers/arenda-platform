import { BooleanField, Datagrid, DateField, List, SelectField, SelectInput, Show, SimpleShowLayout, TextField, TextInput } from 'react-admin';
import {
  ChoiceChipField,
  LeaseReferenceField,
  MoneyField,
  PropertyLinkField,
  PropertyReferenceField,
  UserReferenceField,
  operationStatusChoices,
  operationTypeChoices,
} from './fields';

const operationFilters = [
  <TextInput key="q" source="q" label="Поиск" alwaysOn />,
  <SelectInput key="status" source="status" label="Статус" choices={operationStatusChoices} />,
  <SelectInput key="type" source="type" label="Тип" choices={operationTypeChoices} />,
];

// Колонки операций — общие для OperationList и вкладок «Операции» на Show-страницах.
// sortable={false} проставлен всем колонкам вне whitelist сортировки бэкенда
// (operations: operationDate, amountKopecks, status) — иначе бэкенд отвечает 400.
export const OperationDatagrid = () => (
  <Datagrid rowClick="show" bulkActionButtons={false}>
    <DateField source="operationDate" />
    <TextField source="name" sortable={false} />
    <TextField source="categoryName" sortable={false} />
    <PropertyLinkField source="propertyName" sortable={false} />
    <SelectField source="type" choices={operationTypeChoices} sortable={false} />
    <ChoiceChipField source="status" choices={operationStatusChoices} />
    <MoneyField source="amountKopecks" />
  </Datagrid>
);

export const OperationList = () => (
  <List filters={operationFilters} sort={{ field: 'operationDate', order: 'DESC' }}>
    <OperationDatagrid />
  </List>
);

export const OperationShow = () => (
  <Show>
    <SimpleShowLayout>
      <UserReferenceField source="ownerId" />
      <PropertyReferenceField source="propertyId" />
      <LeaseReferenceField source="leaseId" />
      <SelectField source="type" choices={operationTypeChoices} />
      <TextField source="categoryName" />
      <TextField source="name" />
      <MoneyField source="amountKopecks" />
      <DateField source="operationDate" />
      <SelectField source="status" choices={operationStatusChoices} />
      <TextField source="comment" />
      <BooleanField source="isException" />
      <TextField source="reminderOffsetDays" />
      <DateField source="createdAt" showTime />
      <DateField source="updatedAt" showTime />
      <TextField source="recurringOperationId" />
      <TextField source="id" />
    </SimpleShowLayout>
  </Show>
);
