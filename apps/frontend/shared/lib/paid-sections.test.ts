import { describe, expect, it } from 'vitest';
import { paidSectionGateRedirect, sharingGateRedirect } from './paid-sections';

const EXPECTED_TARIFF_CHANGE_REDIRECT = '/profile/tariff/change';
const EXPECTED_SHARING_INTERSTITIAL = '/profile/tariff/sharing';

describe('paidSectionGateRedirect', () => {
    it('does not gate the per-object participants routes on any tariff (карта #1459): участник читает на basic, владельца разводит экран — proxy роль не знает', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants', 'basic'),
        ).toBeNull();
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants/invite', 'basic'),
        ).toBeNull();
        expect(
            paidSectionGateRedirect(
                '/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants/99999999-9999-4999-8999-999999999931',
                null,
            ),
        ).toBeNull();
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants', 'pro'),
        ).toBeNull();
    });

    it('does not gate the property detail page', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001', 'basic'),
        ).toBeNull();
    });

    it('opens the section page itself on the basic tariff (D2 #1313: upsell empty state)', () => {
        expect(paidSectionGateRedirect('/participants', 'basic')).toBeNull();
    });

    it('gates deep routes of the global participants section, including search and the invite form', () => {
        expect(paidSectionGateRedirect('/participants/search', 'basic')).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
        expect(paidSectionGateRedirect('/participants/invite', 'basic')).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
        expect(
            paidSectionGateRedirect('/participants/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001', 'basic'),
        ).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
        expect(
            paidSectionGateRedirect(
                '/participants/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0002',
                'basic',
            ),
        ).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
    });

    it('gates the history feed, and nothing else of the history section (карта #1459 #1466): объектные поверхности выпускают basic — разводку делает экран, proxy роль не знает', () => {
        expect(paidSectionGateRedirect('/history', 'basic')).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
        expect(paidSectionGateRedirect('/history/', 'basic')).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
        expect(
            paidSectionGateRedirect('/history/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001', 'basic'),
        ).toBeNull();
        expect(
            paidSectionGateRedirect(
                '/history/participants/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0002',
                'basic',
            ),
        ).toBeNull();
        expect(
            paidSectionGateRedirect(
                '/history/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001',
                null,
            ),
        ).toBeNull();
        expect(
            paidSectionGateRedirect(
                '/history/participants/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0002',
                null,
            ),
        ).toBeNull();
    });

    it('passes a paid tariff through', () => {
        expect(paidSectionGateRedirect('/participants', 'pro')).toBeNull();
        expect(paidSectionGateRedirect('/history', 'pro')).toBeNull();
        expect(
            paidSectionGateRedirect('/history/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001', 'business'),
        ).toBeNull();
    });

    it('gates a missing subscription like the basic tariff', () => {
        expect(paidSectionGateRedirect('/participants/search', null)).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
        expect(paidSectionGateRedirect('/history', null)).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
    });

    it('is a no-op outside the gated routes regardless of the tariff', () => {
        expect(paidSectionGateRedirect('/payments', 'basic')).toBeNull();
        expect(paidSectionGateRedirect('/profile/tariff/change', null)).toBeNull();
    });
});

describe('sharingGateRedirect (screen-разводка #1463, ADR 0064 регистр 2)', () => {
    it('sends the owner on the basic tariff to the sharing interstitial (#1470)', () => {
        expect(sharingGateRedirect(true, false)).toBe(EXPECTED_SHARING_INTERSTITIAL);
    });

    it('keeps the owner on a paid tariff on the participants page', () => {
        expect(sharingGateRedirect(true, true)).toBeNull();
    });

    it('lets a participant use the page on any tariff — бекенд-гейт роль-осведомлённый (#1462)', () => {
        expect(sharingGateRedirect(false, false)).toBeNull();
        expect(sharingGateRedirect(false, true)).toBeNull();
    });
});
