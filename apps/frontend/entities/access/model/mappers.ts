import type { components } from '@/shared/api/dto';
import type { PropertyAccessMember } from './types';

type PropertyAccessMemberResponse =
    components['schemas']['PropertyAccessMemberResponse'];

export function mapPropertyAccessMemberResponse(
    dto: PropertyAccessMemberResponse,
): PropertyAccessMember {
    return {
        id: dto.id ?? null,
        userId: dto.user_id ?? null,
        email: dto.email ?? null,
        role: dto.role,
        isOwner: dto.is_owner,
        displayName: dto.display_name ?? '',
        hasEmail: dto.has_email ?? false,
        status: dto.status,
        suspendedAt: dto.suspended_at ?? null,
        lastSentAt: dto.last_sent_at ?? null,
    };
}
