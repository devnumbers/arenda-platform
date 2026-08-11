export const reminderKeys = {
  all: ['reminders'] as const,
  list: (limit: number, offset: number) =>
    [...reminderKeys.all, 'list', limit, offset] as const,
  detail: (id: string) => [...reminderKeys.all, id] as const,
  calendar: (from: string, to: string) =>
    [...reminderKeys.all, 'calendar', from, to] as const,
};
