import { Admin, Resource } from 'react-admin';
import { authProvider } from './authProvider';
import { dataProvider } from './dataProvider';
import { i18nProvider } from './i18n';
import { LoginPage } from './LoginPage';
import { UserList, UserShow } from './users';
import { SubscriptionPaymentList, SubscriptionPaymentShow } from './subscriptionPayments';
import { PropertyShow } from './properties';
import { LeaseShow } from './leases';
import { TenantContactShow } from './tenantContacts';
import { OperationShow } from './operations';

export default function App() {
  return (
    <Admin
      authProvider={authProvider}
      dataProvider={dataProvider}
      i18nProvider={i18nProvider}
      loginPage={LoginPage}
    >
      <Resource name="users" list={UserList} show={UserShow} />
      <Resource name="subscriptionPayments" list={SubscriptionPaymentList} show={SubscriptionPaymentShow} />
      <Resource name="properties" show={PropertyShow} />
      <Resource name="leases" show={LeaseShow} />
      <Resource name="tenantContacts" show={TenantContactShow} />
      <Resource name="operations" show={OperationShow} />
    </Admin>
  );
}
