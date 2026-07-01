import { BooleanField, DateField, Show, SimpleShowLayout, TextField } from 'react-admin';

export const OperationShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField source="id" />
      <TextField source="ownerId" />
      <TextField source="propertyId" />
      <TextField source="leaseId" />
      <TextField source="recurringOperationId" />
      <TextField source="type" />
      <TextField source="category" />
      <TextField source="name" />
      <TextField source="amountKopecks" />
      <DateField source="operationDate" />
      <TextField source="status" />
      <TextField source="comment" />
      <BooleanField source="isException" />
      <TextField source="reminderOffsetDays" />
      <DateField source="createdAt" />
      <DateField source="updatedAt" />
    </SimpleShowLayout>
  </Show>
);
