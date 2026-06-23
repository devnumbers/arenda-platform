export const propertyKeys = {
  all: ['properties'] as const,
  detail: (id: string) => [...propertyKeys.all, id] as const,
};
