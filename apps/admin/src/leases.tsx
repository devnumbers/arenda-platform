import {
  Datagrid,
  DateField,
  List,
  ReferenceManyField,
  SelectField,
  SelectInput,
  Show,
  Tab,
  TabbedShowLayout,
  TextField,
} from 'react-admin';
import {
  ChoiceChipField,
  MoneyField,
  PropertyLinkField,
  PropertyReferenceField,
  TenantContactField,
  UserReferenceField,
  leaseStatusChoices,
} from './fields';
import { OperationDatagrid } from './operations';

const leaseFilters = [
  <SelectInput key="status" source="status" label="Статус" choices={leaseStatusChoices} />,
];

// Колонки договоров — общие для LeaseList и вкладки «Договоры» в PropertyShow.
// sortable={false} проставлен всем колонкам вне whitelist сортировки бэкенда
// (leases: startDate, updatedAt, status, rentAmountKopecks) — иначе бэкенд отвечает 400.
export const LeaseDatagrid = () => (
  <Datagrid rowClick="show" bulkActionButtons={false}>
    <PropertyLinkField source="propertyName" sortable={false} />
    <TenantContactField source="tenantContact" sortable={false} />
    <ChoiceChipField source="status" choices={leaseStatusChoices} />
    <MoneyField source="rentAmountKopecks" />
    <DateField source="startDate" />
    <DateField source="endDate" sortable={false} />
  </Datagrid>
);

export const LeaseList = () => (
  <List filters={leaseFilters} sort={{ field: 'updatedAt', order: 'DESC' }}>
    <LeaseDatagrid />
  </List>
);

export const LeaseShow = () => (
  <Show>
    <TabbedShowLayout>
      <Tab label="Договор">
        <UserReferenceField source="ownerId" />
        <PropertyReferenceField source="propertyId" />
        <TenantContactField source="tenantContact" />
        <TextField source="tenantContact.email" />
        <SelectField source="status" choices={leaseStatusChoices} />
        <DateField source="startDate" />
        <DateField source="endDate" />
        <MoneyField source="rentAmountKopecks" />
        <MoneyField source="depositAmountKopecks" />
        <TextField source="paymentDay" />
        <TextField source="comment" />
        <DateField source="createdAt" showTime />
        <DateField source="updatedAt" showTime />
        <TextField source="id" />
      </Tab>
      <Tab label="Операции">
        <ReferenceManyField
          reference="operations"
          target="lease_id"
          label={false}
          sort={{ field: 'operationDate', order: 'DESC' }}
        >
          <OperationDatagrid />
        </ReferenceManyField>
      </Tab>
    </TabbedShowLayout>
  </Show>
);
