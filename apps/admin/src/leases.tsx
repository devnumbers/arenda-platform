import { DateField, Show, SimpleShowLayout, TextField } from 'react-admin';

export const LeaseShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField source="id" />
      <TextField source="ownerId" />
      <TextField source="propertyId" />
      <TextField source="tenantContactId" />
      <TextField source="status" />
      <DateField source="startDate" />
      <DateField source="endDate" />
      <TextField source="rentAmountKopecks" />
      <TextField source="depositAmountKopecks" />
      <TextField source="paymentDay" />
      <TextField source="comment" />
      <DateField source="createdAt" />
      <DateField source="updatedAt" />
    </SimpleShowLayout>
  </Show>
);
