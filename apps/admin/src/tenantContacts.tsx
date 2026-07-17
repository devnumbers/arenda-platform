import { Datagrid, DateField, List, Show, SimpleShowLayout, TextField, TextInput } from 'react-admin';
import { FullNameField, OwnerLinkField, UserReferenceField } from './fields';

const tenantContactFilters = [<TextInput key="q" source="q" label="Поиск" alwaysOn />];

// sortable={false} проставлен всем колонкам: whitelist сортировки бэкенда для
// tenant-contacts (name, updatedAt) не покрывает отображаемые поля — иначе бэкенд отвечает 400.
export const TenantContactList = () => (
  <List filters={tenantContactFilters} sort={{ field: 'name', order: 'ASC' }}>
    <Datagrid rowClick="show" bulkActionButtons={false}>
      <FullNameField source="name" label="ФИО" sortable={false} />
      <TextField source="phone" sortable={false} />
      <TextField source="email" sortable={false} />
      <OwnerLinkField source="ownerId" sortable={false} />
    </Datagrid>
  </List>
);

export const TenantContactShow = () => (
  <Show>
    <SimpleShowLayout>
      <UserReferenceField source="ownerId" />
      <TextField source="name" />
      <TextField source="surname" />
      <TextField source="patronymic" />
      <TextField source="phone" />
      <TextField source="email" />
      <TextField source="comment" />
      <DateField source="createdAt" showTime />
      <DateField source="updatedAt" showTime />
      <TextField source="id" />
    </SimpleShowLayout>
  </Show>
);
