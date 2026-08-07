export type AccessRole = 'owner' | 'full_access' | 'viewer';

/** Роли участника совместного доступа (владелец не является membership). */
export type SharedAccessRole = Exclude<AccessRole, 'owner'>;

export type AccessMemberStatus = 'active' | 'suspended' | 'pending';

export type PropertyAccessMember = {
    readonly id: string | null;
    readonly userId: string | null;
    readonly email: string | null;
    readonly role: AccessRole;
    readonly isOwner: boolean;
    readonly displayName: string;
    readonly hasEmail: boolean;
    readonly status: AccessMemberStatus;
    readonly suspendedAt?: string | null;
    readonly lastSentAt?: string | null;
};
