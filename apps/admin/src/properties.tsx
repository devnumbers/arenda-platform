import { DateField, Show, SimpleShowLayout, TextField } from 'react-admin';

export const PropertyShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField source="id" />
      <TextField source="ownerId" />
      <TextField source="name" />
      <TextField source="type" />
      <TextField source="status" />
      <TextField source="occupancy" />
      <TextField source="address" />
      <TextField source="description" />
      <DateField source="createdAt" />
      <DateField source="updatedAt" />
    </SimpleShowLayout>
  </Show>
);
