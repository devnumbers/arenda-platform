export const endpoints = {
  get_me: {
    method: 'GET',
    path: '/api/v1/users/me',
    expectedStatuses: [200, 401],
  },
};
