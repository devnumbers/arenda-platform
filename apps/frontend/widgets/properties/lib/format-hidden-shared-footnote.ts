import {pluralize} from '@/shared/lib/pluralize';

/**
 * Форматирует сноску о скрытых общих объектах: согласует с числом
 * и существительное («общий объект»), и краткое прилагательное («скрыт»).
 */
export function formatHiddenSharedFootnote(count: number): string {
    const noun = pluralize(count, 'общий объект', 'общих объекта', 'общих объектов');
    const verb = pluralize(count, 'скрыт', 'скрыто', 'скрыто');
    return `${count} ${noun} ${verb} — превышен лимит объектов.`;
}
