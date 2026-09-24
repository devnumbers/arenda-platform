import type { components } from '@/shared/api/dto';
import type {
  Rental,
  RentalNextPayment,
  RentalPaymentView,
  RentalProgress,
  RentalSummary,
  RentalTenant,
} from './types';

type RentalResponseDto = components['schemas']['RentalResponse'];
type RentalSummaryDto = components['schemas']['RentalSummaryResponse'];
type TenantViewDto = components['schemas']['TenantView'];
type NextPaymentDto = NonNullable<
  components['schemas']['RentalPaymentView']['nextPayment']
>;

/** Итоги аренды (решение №13): сервер отдаёт период и суммы каноническими —
 * переносится как есть. */
export function mapRentalSummary(dto: RentalSummaryDto): RentalSummary {
  return {
    from: dto.from,
    until: dto.until,
    incomeKopecks: dto.incomeKopecks,
    expenseKopecks: dto.expenseKopecks,
    profitKopecks: dto.profitKopecks,
  };
}

/** DTO → сущность: отсутствующие в проводе необязательные поля нормализуются
 * в null (tenant, plannedEndDate, nextPayment), остальное переносится как
 * есть — даты и копейки сервер отдаёт уже каноническими (ADR 0053 §4). */
export function mapRental(dto: RentalResponseDto): Rental {
  return {
    id: dto.id,
    propertyId: dto.propertyId,
    status: dto.status,
    startDate: dto.startDate,
    plannedEndDate: dto.plannedEndDate ?? null,
    completedDate: dto.completedDate,
    utilities: dto.utilities,
    depositKopecks: dto.depositKopecks,
    commissionKopecks: dto.commissionKopecks,
    depositReturnKopecks: dto.depositReturnKopecks,
    depositReturnComment: dto.depositReturnComment,
    tenant: mapTenant(dto.tenant),
    comment: dto.comment,
    rentPayment: mapRentPayment(dto.rentPayment),
    progress: mapProgress(dto.progress),
    today: dto.today,
    createdAt: dto.createdAt,
  };
}

function mapTenant(dto: TenantViewDto | undefined): RentalTenant | null {
  return dto === undefined
    ? null
    : {
        contactId: dto.contactId,
        firstName: dto.firstName,
        lastName: dto.lastName,
        phone: dto.phone,
      };
}

function mapRentPayment(dto: components['schemas']['RentalPaymentView']): RentalPaymentView {
  return {
    paymentId: dto.paymentId,
    amountKopecks: dto.amountKopecks,
    paymentDay: dto.paymentDay,
    autoPay: dto.autoPay,
    nextPayment: mapNextPayment(dto.nextPayment),
  };
}

function mapNextPayment(dto: NextPaymentDto | undefined): RentalNextPayment | null {
  return dto === undefined
    ? null
    : {
        operationId: dto.operationId,
        date: dto.date,
        amountKopecks: dto.amountKopecks,
        daysUntil: dto.daysUntil,
      };
}

function mapProgress(dto: components['schemas']['RentalProgress']): RentalProgress {
  return {
    paidMonths: dto.paidMonths,
    totalMonths: dto.totalMonths,
    monthsRemaining: dto.monthsRemaining,
    overdueMonths: dto.overdueMonths,
  };
}
