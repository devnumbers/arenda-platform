import { DateField, Show, SimpleShowLayout, TextField } from 'react-admin';

export const TenantContactShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField source="id" />
      <TextField source="ownerId" />
      <TextField source="name" />
      <TextField source="surname" />
      <TextField source="patronymic" />
      <TextField source="phone" />
      <TextField source="email" />
      <TextField source="comment" />
      <DateField source="createdAt" />
      <DateField source="updatedAt" />
    </SimpleShowLayout>
  </Show>
);
