export const reminderKeys = {
  all: ['reminders'] as const,
  detail: (id: string) => [...reminderKeys.all, id] as const,
};
