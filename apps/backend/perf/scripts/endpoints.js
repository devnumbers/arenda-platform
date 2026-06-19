function sessionCookie(index) {
  return `session_id=perf-session-token-${index}`;
}

function ownerIndex(ctx) {
  return (ctx.vu + ctx.iteration) % 1000;
}

export const endpoints = {
  get_me: {
    request: (ctx) => ({
      method: 'GET',
      url: `${ctx.baseUrl}/me`,
      headers: { Cookie: sessionCookie(ownerIndex(ctx)) },
    }),
    expectedStatuses: [200],
  },
};
