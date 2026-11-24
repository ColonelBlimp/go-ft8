# Release notes

## v0.10.0 — 2026-11-24

### Compatibility and migration

- **FT4 decoder sharing changes:** in v0.9.0, `ft4.Decoder` stored only options.
  It now retains mutable callsign hash history and is not safe for concurrent
  method calls. Allocate one decoder per receiver stream and process that
  stream's slots sequentially. Sharing an instance across streams or bands mixes
  their histories even if callers serialize access. Create a new decoder on a
  band change when history should start empty. Package-level FT4 decode
  functions remain stateless and safe for concurrent calls.
- FT4 decoder instances now render a known hash as `<CALL>` instead of `<...>`.
- Automatic history learning in both modes now excludes protocol tokens such as
  `QRZ`, suffixed CQ tokens, reserved call-field values, and type 4 text with
  embedded spaces. This corrects pre-existing FT8 behavior; those non-call
  tokens no longer resolve subsequent hashes. Legacy message text parsing
  remains permissive and continues to expose the original verified payload.

### Added

- Per-instance FT4 history for 10-, 12-, and 22-bit callsign hashes, using FT8's
  learning fields, candidate order, capacity, and recency rules.
- `RememberCallsigns` on both decoders for known calls, independent of AP hints.
- `ft8.CallsignHistory` for stateful rendering of verified FT4/FT8 payloads.
- `ft8.CallsignHash` and `Payload77.DXpedition()` for comparing a type 0.1
  sender's raw hash with a known call. Callsign inputs must be ASCII; the
  accessor rejects special/reserved recipient tokens.

Hash matching retains the existing newest-match policy. A matching hash is a
possible identity, including when multiple retained calls match or an unheard
sender collides with a retained call; it is not unique proof of identity.
