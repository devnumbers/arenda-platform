export const accessKeys = {
    all: ['property-access'] as const,
    list: (propertyId: string) =>
        [...accessKeys.all, 'list', propertyId] as const,
};
