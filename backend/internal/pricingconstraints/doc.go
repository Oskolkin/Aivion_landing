// Package pricingconstraints contains domain formulas and validation primitives
// for pricing constraints.
//
// Deprecated/future module: not part of active seller-facing MVP. The user-facing
// pricing constraints screen was removed; backend APIs remain for future use and tests.
//
// Margin semantics are fixed across this package:
//   - margin = (price - cost) / price
//   - margin is stored and passed as decimal fraction (0.25, not "25%").
package pricingconstraints
