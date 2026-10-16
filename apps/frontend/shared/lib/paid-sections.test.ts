import { describe, expect, it } from 'vitest';
import { paidSectionGateRedirect } from './paid-sections';

const EXPECTED_TARIFF_CHANGE_REDIRECT = '/profile/tariff/change';

describe('paidSectionGateRedirect', () => {
    it('gates the property participants list on the basic tariff', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants', 'basic'),
        ).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
    });

    it('gates deep routes of the section, e.g. the invite page', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants/invite', 'basic'),
        ).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
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

    it('gates the history journal on the basic tariff', () => {
        expect(paidSectionGateRedirect('/history', 'basic')).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
    });

    it('gates deep routes of the history section, both nested entities', () => {
        expect(
            paidSectionGateRedirect('/history/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001', 'basic'),
        ).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
        expect(
            paidSectionGateRedirect(
                '/history/participants/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0002',
                'basic',
            ),
        ).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
    });

    it('passes a paid tariff through', () => {
        expect(paidSectionGateRedirect('/participants', 'pro')).toBeNull();
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants', 'pro'),
        ).toBeNull();
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants/invite', 'business'),
        ).toBeNull();
        expect(paidSectionGateRedirect('/history', 'pro')).toBeNull();
        expect(
            paidSectionGateRedirect('/history/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001', 'business'),
        ).toBeNull();
    });

    it('gates a missing subscription like the basic tariff', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants', null),
        ).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
        expect(paidSectionGateRedirect('/participants/search', null)).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
        expect(paidSectionGateRedirect('/history', null)).toBe(EXPECTED_TARIFF_CHANGE_REDIRECT);
    });

    it('is a no-op outside the gated routes regardless of the tariff', () => {
        expect(paidSectionGateRedirect('/payments', 'basic')).toBeNull();
        expect(paidSectionGateRedirect('/profile/tariff/change', null)).toBeNull();
    });
});
