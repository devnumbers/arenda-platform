import { describe, expect, it } from 'vitest';
import { paidSectionGateRedirect } from './paid-sections';

const EXPECTED_PARTICIPANTS_GATE_REDIRECT = '/profile/tariff/change?gate=participants';

describe('paidSectionGateRedirect', () => {
    it('gates the property participants list on the basic tariff', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants', 'basic'),
        ).toBe('/profile/tariff/change?gate=property-participants');
    });

    it('gates deep routes of the section, e.g. the invite page', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants/invite', 'basic'),
        ).toBe('/profile/tariff/change?gate=property-participants');
    });

    it('does not gate the property detail page', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001', 'basic'),
        ).toBeNull();
    });

    it('gates the global participants section on the basic tariff', () => {
        expect(paidSectionGateRedirect('/participants', 'basic')).toBe(EXPECTED_PARTICIPANTS_GATE_REDIRECT);
    });

    it('gates deep routes of the global participants section, including the invite form', () => {
        expect(paidSectionGateRedirect('/participants/list', 'basic')).toBe(EXPECTED_PARTICIPANTS_GATE_REDIRECT);
        expect(paidSectionGateRedirect('/participants/properties', 'basic')).toBe(EXPECTED_PARTICIPANTS_GATE_REDIRECT);
        expect(paidSectionGateRedirect('/participants/invite', 'basic')).toBe(EXPECTED_PARTICIPANTS_GATE_REDIRECT);
        expect(
            paidSectionGateRedirect('/participants/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001', 'basic'),
        ).toBe(EXPECTED_PARTICIPANTS_GATE_REDIRECT);
    });

    it('passes a paid tariff through', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants', 'pro'),
        ).toBeNull();
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants/invite', 'business'),
        ).toBeNull();
    });

    it('gates a missing subscription like the basic tariff', () => {
        expect(
            paidSectionGateRedirect('/properties/0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001/participants', null),
        ).toBe('/profile/tariff/change?gate=property-participants');
    });

    it('is a no-op outside the gated routes regardless of the tariff', () => {
        expect(paidSectionGateRedirect('/payments', 'basic')).toBeNull();
        expect(paidSectionGateRedirect('/profile/tariff/change', null)).toBeNull();
    });
});
