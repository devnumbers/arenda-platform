import {
  ArrayField,
  Datagrid,
  DateField,
  ImageField,
  List,
  ReferenceManyField,
  SelectField,
  SelectInput,
  Show,
  SingleFieldList,
  Tab,
  TabbedShowLayout,
  TextField,
  TextInput,
} from 'react-admin';
import {
  ChoiceChipField,
  OwnerLinkField,
  UserReferenceField,
  occupancyChoices,
  propertyStatusChoices,
  propertyStatusFilterChoices,
  propertyTypeChoices,
} from './fields';
import { LeaseDatagrid } from './leases';
import { OperationDatagrid } from './operations';

const propertyFilters = [
  <TextInput key="q" source="q" label="Поиск" alwaysOn />,
  <SelectInput key="status" source="status" label="Статус" choices={propertyStatusFilterChoices} />,
];

// sortable={false} проставлен всем колонкам вне whitelist сортировки бэкенда
// (properties: name, createdAt, updatedAt, status) — иначе бэкенд отвечает 400.
export const PropertyList = () => (
  <List filters={propertyFilters} sort={{ field: 'name', order: 'ASC' }}>
    <Datagrid rowClick="show" bulkActionButtons={false}>
      <TextField source="name" />
      <TextField source="address" sortable={false} />
      <SelectField source="type" choices={propertyTypeChoices} sortable={false} />
      <ChoiceChipField source="status" choices={propertyStatusChoices} />
      <SelectField source="occupancy" choices={occupancyChoices} sortable={false} />
      <OwnerLinkField source="ownerId" sortable={false} />
    </Datagrid>
  </List>
);

export const PropertyShow = () => (
  <Show>
    <TabbedShowLayout>
      <Tab label="Объект">
        <UserReferenceField source="ownerId" />
        <TextField source="name" />
        <SelectField source="type" choices={propertyTypeChoices} />
        <SelectField source="status" choices={propertyStatusChoices} />
        <SelectField source="occupancy" choices={occupancyChoices} />
        <TextField source="address" />
        <TextField source="description" />
        <ArrayField source="photos">
          <SingleFieldList linkType={false}>
            <ImageField source="url" />
          </SingleFieldList>
        </ArrayField>
        <DateField source="createdAt" showTime />
        <DateField source="updatedAt" showTime />
        <TextField source="id" />
      </Tab>
      <Tab label="Договоры">
        <ReferenceManyField
          reference="leases"
          target="property_id"
          label={false}
          sort={{ field: 'updatedAt', order: 'DESC' }}
        >
          <LeaseDatagrid />
        </ReferenceManyField>
      </Tab>
      <Tab label="Операции">
        <ReferenceManyField
          reference="operations"
          target="property_id"
          label={false}
          sort={{ field: 'operationDate', order: 'DESC' }}
        >
          <OperationDatagrid />
        </ReferenceManyField>
      </Tab>
    </TabbedShowLayout>
  </Show>
);
