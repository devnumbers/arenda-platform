package domain

import "testing"

// testMaskVisa is the masked Visa card shared by the card-system and
// card-snapshot tests.
const testMaskVisa = "4300********1234"

// TestCardSystemFromMask proves the display-mask BIN derivation behind the
// contract's cardSystem field (issue #619): 2 — Mir, 4 — Visa,
// 5 — Mastercard, everything else unknown. The contract enum has no
// UnionPay value, so a 62-prefixed BIN is unknown too.
func TestCardSystemFromMask(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mask string
		want CardSystem
	}{
		{mask: "2202********1234", want: CardSystemMir},
		{mask: testMaskVisa, want: CardSystemVisa},
		{mask: "5100********1234", want: CardSystemMastercard},
		{mask: "5536********1234", want: CardSystemMastercard},
		{mask: "6200********1234", want: CardSystemUnknown},
		{mask: "1234********1234", want: CardSystemUnknown},
		{mask: "********1234", want: CardSystemUnknown},
		{mask: "", want: CardSystemUnknown},
	}
	for _, tc := range cases {
		if got := CardSystemFromMask(tc.mask); got != tc.want {
			t.Errorf("CardSystemFromMask(%q) = %q, want %q", tc.mask, got, tc.want)
		}
	}
}
