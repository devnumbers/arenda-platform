export type AccessRole = 'owner' | 'full_access' | 'viewer';

export type PropertyAccessMember = {
    readonly id: string | null;
    readonly userId: string;
    readonly role: AccessRole;
    readonly isOwner: boolean;
    readonly displayName: string;
    readonly hasEmail: boolean;
};
