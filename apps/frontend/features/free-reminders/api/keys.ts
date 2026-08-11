export const freeReminderKeys = {
  all: ['free-reminders'] as const,
  detail: (id: string) => [...freeReminderKeys.all, 'detail', id] as const,
  byProperty: (propertyId: string) =>
    [...freeReminderKeys.all, 'by-property', propertyId] as const,
  upcoming: (propertyId: string) =>
    [...freeReminderKeys.all, 'upcoming', propertyId] as const,
};
