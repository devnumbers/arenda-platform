package application

import "errors"

// ErrTariffRequired is the role-aware tariff gate verdict (ADR 0064, ревизия
// #1462): the actor is the property owner, but their tariff does not cover
// the paid sections. The HTTP adapters map it to 402 PaymentRequired with the
// machine code "tariff_required" — the wire shape the route middleware wrote
// before the gate moved into the use cases. A property member never hears
// this verdict: their reads are the «пригласили — значит пользуется» rule and
// their mutations are refused by the role policy before the gate.
var ErrTariffRequired = errors.New("paid tariff required")
