package httpsupport

import "net/url"

// Frontend return routes of backend redirects. They mirror existing
// frontend routes (ROUTES in apps/frontend) — one copy per wire contract:
// the add-card return is shared by the T-Kassa binding return (httpserver)
// and the fake provider autoconfirm (billing) so both providers land the
// browser on the same screen with the same flag (issue #663).
const (
	// AddCardReturnPath — «Способы оплаты»: its mount effect consumes the
	// addCard flag.
	AddCardReturnPath = "/profile/tariff/payment-methods"

	// AddCardResultSuccess and AddCardResultFailed are the values of the
	// addCard query flag the screen's mount effect reads.
	AddCardResultSuccess = "success"
	AddCardResultFailed  = "fail"
)

// AddCardReturnURL builds the absolute frontend redirect target of a
// completed card binding: AddCardReturnPath keyed by the addCard flag.
func AddCardReturnURL(baseURL, result string) string {
	target := baseURL
	if path, err := url.JoinPath(baseURL, AddCardReturnPath); err == nil {
		target = path
	}
	q := url.Values{"addCard": {result}}
	return target + "?" + q.Encode()
}
