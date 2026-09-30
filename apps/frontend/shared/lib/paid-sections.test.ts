import { describe, expect, it } from 'vitest';
import { paidSectionGateRedirect } from './paid-sections';

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

    it('does not gate look-alike sections such as the global participants hub', () => {
        expect(paidSectionGateRedirect('/participants/list', 'basic')).toBeNull();
        expect(paidSectionGateRedirect('/participants/properties', 'basic')).toBeNull();
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
