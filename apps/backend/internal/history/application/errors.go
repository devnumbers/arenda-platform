package application

import "errors"

// The contract's read errors: ErrInvalidInput is the 400 (malformed cursor,
// unknown vocabulary, bad page size, oversized query), ErrNotFound — the
// privacy 404 (an invisible or unknown property id: the request's existence
// is not revealed). ErrTariffRequired is the role-aware tariff gate verdict
// (ADR 0064, ревизия #1465): on a non-paid tariff the reads pass only in the
// narrowed membership scope — the HTTP adapters answer it with the canonical
// 402 tariff_required.
var (
	ErrInvalidInput   = errors.New("history: invalid input")
	ErrNotFound       = errors.New("history: not found")
	ErrTariffRequired = errors.New("paid tariff required")
)
