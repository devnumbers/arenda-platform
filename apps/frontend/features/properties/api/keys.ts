export const propertyKeys = {
  all: ['properties'] as const,
  list: ['properties', 'list'] as const,
  detail: (id: string) => ['properties', 'detail', id] as const,
  addressSuggestions: (query: string) =>
    [...propertyKeys.all, 'address-suggestions', query] as const,
};
