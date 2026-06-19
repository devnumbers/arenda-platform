const fixtures = JSON.parse(open('../../perf/fixtures.json'));

function sessionCookie(owner) {
  return `session_id=${owner.sessionToken}`;
}

function ownerByIndex(ctx) {
  const i = (ctx.vu + ctx.iteration) % fixtures.owners.length;
  return fixtures.owners[i];
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
};
