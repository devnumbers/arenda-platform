const fixtures = JSON.parse(open('../../perf/fixtures.json'));

function sessionCookie(owner) {
  return `session_id=${owner.sessionToken}`;
}

function ownerByIndex(ctx) {
  const i = (ctx.vu + ctx.iteration) % fixtures.owners.length;
  return fixtures.owners[i];
}

function formatDateISO(date) {
  return date.toISOString().split('T')[0];
}

function today() {
  return formatDateISO(new Date());
}

function daysAgo(n) {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return formatDateISO(d);
}

function daysFromNow(n) {
  const d = new Date();
  d.setDate(d.getDate() + n);
  return formatDateISO(d);
}

export const endpoints = {
  get_me: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/me`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_properties: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/properties`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_property: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200, 404],
  },

  list_operations: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/operations`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_operation: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/operations/${owner.operationID}`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200, 404],
  },

  list_recurring_operations: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/recurring-operations`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_recurring_operation: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/recurring-operations/${owner.recurringOperationID}`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200, 404],
  },

  list_leases: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/leases`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_lease: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/leases/${owner.leaseID}`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200, 404],
  },

  list_tenant_contacts: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/tenant-contacts`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_tenant_contact: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/tenant-contacts/${owner.tenantContactID}`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200, 404],
  },

  get_operation_reminders: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/operations/${owner.operationID}/reminders`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_recurring_operation_reminders: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/recurring-operations/${owner.recurringOperationID}/reminders`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_lease_reminders: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/leases/${owner.leaseID}/reminders`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  list_reminders: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/reminders?limit=20&offset=0`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_tariffs: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/tariffs`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  get_subscription: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/subscription`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  list_subscription_payments: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/subscription/payments`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  list_payment_methods: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'GET',
        url: `${ctx.baseUrl}/subscription/payment-methods`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  auth_send_code: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/auth/phone/send`,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone: owner.phone }),
      };
    },
    expectedStatuses: [204, 429],
  },

  auth_verify_code: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/auth/phone/verify`,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone: owner.phone, code: '000000' }),
      };
    },
    expectedStatuses: [200, 401],
  },

  auth_logout: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/auth/logout`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [204, 401],
  },

  post_property: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      const unique = `${owner.index}-${ctx.vu}-${ctx.iteration}`;
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/properties`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: `Property ${unique}`,
          address: `Address ${unique}`,
          type: 'apartment',
        }),
      };
    },
    expectedStatuses: [201, 400, 401, 403],
  },

  patch_property: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'PATCH',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: 'Updated' }),
      };
    },
    expectedStatuses: [200],
  },

  archive_property: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/archive`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200, 403],
  },

  unarchive_property: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/unarchive`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200, 403],
  },

  create_operation: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/operations`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'expense',
          category: 'repair',
          amount_kopecks: 100000,
          operation_date: today(),
        }),
      };
    },
    expectedStatuses: [201],
  },

  update_operation: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'PATCH',
        url: `${ctx.baseUrl}/operations/${owner.operationID}`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ amount_kopecks: 200000 }),
      };
    },
    expectedStatuses: [200],
  },

  delete_operation: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'DELETE',
        url: `${ctx.baseUrl}/operations/${owner.operationID}`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [204, 404],
  },

  create_recurring_operation: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/recurring-operations`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: 'expense',
          category: 'utilities',
          amount_kopecks: 200000,
          start_date: daysAgo(60),
          payment_day: 1,
        }),
      };
    },
    expectedStatuses: [201],
  },

  update_recurring_operation: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'PATCH',
        url: `${ctx.baseUrl}/recurring-operations/${owner.recurringOperationID}`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ amount_kopecks: 250000 }),
      };
    },
    expectedStatuses: [200],
  },

  pause_recurring_operation: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/recurring-operations/${owner.recurringOperationID}/pause`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  resume_recurring_operation: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/recurring-operations/${owner.recurringOperationID}/resume`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },

  create_lease: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/leases`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          property_id: owner.propertyID,
          tenant_contact_id: owner.tenantContactID,
          start_date: today(),
          end_date: daysFromNow(335),
          rent_amount_kopecks: 5000000,
          payment_day: 1,
        }),
      };
    },
    expectedStatuses: [201, 400, 401, 403],
  },

  update_lease: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'PATCH',
        url: `${ctx.baseUrl}/leases/${owner.leaseID}`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ rent_amount_kopecks: 5500000 }),
      };
    },
    expectedStatuses: [200],
  },

  complete_lease: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/leases/${owner.leaseID}/complete`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200, 401, 403, 404],
  },

  create_tenant_contact: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      const suffix = `${owner.index}-${ctx.vu}-${ctx.iteration}`;
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/tenant-contacts`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: `Contact ${suffix}`,
          phone: `+7955${String(owner.index).padStart(4, '0')}${String(ctx.iteration).padStart(4, '0')}`,
        }),
      };
    },
    expectedStatuses: [201, 409],
  },

  update_tenant_contact: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'PATCH',
        url: `${ctx.baseUrl}/tenant-contacts/${owner.tenantContactID}`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: 'Updated' }),
      };
    },
    expectedStatuses: [200],
  },

  create_operation_reminder: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/operations/${owner.operationID}/reminders`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ reminder_date: daysFromNow(1) }),
      };
    },
    expectedStatuses: [201, 400],
  },

  create_recurring_operation_reminder: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/properties/${owner.propertyID}/recurring-operations/${owner.recurringOperationID}/reminders`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ reminder_date: daysFromNow(1) }),
      };
    },
    expectedStatuses: [201, 400],
  },

  update_reminder: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'PATCH',
        url: `${ctx.baseUrl}/reminders/${owner.reminderID}`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ reminder_date: daysFromNow(2) }),
      };
    },
    expectedStatuses: [200, 400, 409],
  },

  delete_reminder: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      const reminders = owner.extraReminderIDs && owner.extraReminderIDs.length > 0
        ? owner.extraReminderIDs
        : [owner.reminderID];
      const index = (ctx.vu + ctx.iteration) % reminders.length;
      return {
        method: 'DELETE',
        url: `${ctx.baseUrl}/reminders/${reminders[index]}`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [204, 404],
  },

  patch_subscription_auto_renew: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'PATCH',
        url: `${ctx.baseUrl}/subscription/auto-renew`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: true }),
      };
    },
    expectedStatuses: [204, 409],
  },

  cancel_subscription: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/subscription/cancel`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [204, 409],
  },

  change_subscription: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/subscription/change`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ tariffName: 'basic', period: 'month' }),
      };
    },
    expectedStatuses: [200, 400, 409],
  },

  add_payment_method: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/subscription/payment-methods`,
        headers: { Cookie: sessionCookie(owner), 'Content-Type': 'application/json' },
        body: JSON.stringify({ providerToken: `fake-token-${owner.index}-${ctx.iteration}` }),
      };
    },
    expectedStatuses: [201, 409],
  },

  delete_payment_method: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'DELETE',
        url: `${ctx.baseUrl}/subscription/payment-methods/${owner.paymentMethodID}`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [204, 404, 409],
  },

  activate_payment_method: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/subscription/payment-methods/${owner.paymentMethodID}/activate`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [204],
  },

  confirm_fake_subscription_payment: {
    request: (ctx) => {
      const owner = ownerByIndex(ctx);
      return {
        method: 'POST',
        url: `${ctx.baseUrl}/internal/fake-subscription-payment/${owner.pendingPaymentID}/confirm`,
        headers: { Cookie: sessionCookie(owner) },
      };
    },
    expectedStatuses: [200],
  },
};
