# Resolve hashed callsigns on FT4 with FT8's history rules

Corrected implementation brief against go-ft8 v0.9.0, 24 November 2026.

## Problem

In v0.9.0, FT4 renders every verified payload through
`ft8.ParsePayload77`, which has no decoder hash history. `ft4.Decoder` keeps
only options. A type 0.1 DXpedition message therefore renders its sender's
10-bit hash as `<...>`, even when the same receiver stream has already decoded
the sender's full call. Applications requiring attribution can then drop a
contact. FT8 already keeps hash history across slots in a decoder instance.

Both modes recover the same 77-bit source payload; FT4 descrambles it first.
The defect is missing rendering history, independent of signal decoding.

## 1. Give each FT4 decoder independent callsign history

Use FT8's existing learning rules, rather than learning every full-call field:
successfully parsed standard messages save their full second call, including
`/P` or `/R`; type 4 messages save the spelled-out call. Invalid, unsupported,
and reserved payloads must not update history. Automatic saves use the same
ASCII token validation as public seeds. Standard-message learning also excludes
special/reserved 28-bit call-field values before suffix rendering. This fixes
the pre-existing FT8 bug that could teach `QRZ`, suffixed CQ tokens, or type 4
text with embedded spaces.

Each decoder starts empty and keeps its own table. Instances are not safe for
concurrent use. All instance decode methods use the same history. Silent slots
preserve calls; checked-input errors leave history unchanged. FT4 does not need
an A7 slot-parity counter solely to retain hash history.

Within a slot, saves become visible to subsequent candidates in processing
order, as in FT8. Earlier output is not re-rendered when a later candidate
teaches a call. FT8's parallel workers commit in candidate order; FT4 renders
before its final frequency sort.

Share the parser and bounded hash table through the zero-value-ready exported
`ft8.CallsignHistory` type. Its `ParsePayload77` method uses the same
transactional learning path as FT8. Keep package-level FT4 decoding and
`ft8.ParsePayload77` history-free. Package-level FT8 decoding retains its
existing temporary history within one invocation.

Acceptance:

- A standard message with `DX1ABC` as its second call teaches that call. A later
  type 0.1 message using its 10-bit sender hash renders `<DX1ABC>`.
- The corresponding behavior holds for type 4's 12-bit hash and a 22-bit hash
  in a 28-bit call field.
- A fresh decoder given only the hashed message renders `<...>`.
- Learning through any instance decode method is available to all the others.
- Silent slots and rejected checked requests preserve existing history.
- A call learned earlier in candidate order resolves later candidates;
  learning later does not change text already emitted.
- Existing results without hashed calls and package-level FT4 results remain
  unchanged. FT8 retains its learning fields and recency/collision policy, but
  stops learning non-call tokens; those tokens no longer resolve later hashes.

## 2. Let callers seed rendering history

Both decoder types expose:

```go
func (d *Decoder) RememberCallsigns(calls ...string)
```

The typical seeds are the operator's call and the station being worked. Seeds
are trimmed and uppercased; one surrounding pair of angle brackets is allowed.
Invalid or reserved tokens, non-ASCII input, embedded whitespace, and calls
outside 3-11 base-38 characters are ignored. Non-ASCII bytes are rejected before
uppercasing, so Unicode case folding cannot introduce ASCII lookalikes.
Validation concerns protocol tokens, not issuance. Automatic `Save` uses the
same normalization and validation.

Seeds use `Save`'s existing size cap and collision/recency rules: at most 1000
entries keyed by 22-bit hash, newest matching entry preferred for 10-, 12-, and
22-bit lookups, and repeated saves refreshing recency.

This affects how a decoded hash is printed, without assisting signal decoding.
FT8's `SetAPCallHints` configures signal-decoding hints and does not seed hash
history. Callers can set both independently.

## 3. Expose hashes and type 0.1 sender fields

The shared payload API is in `ft8`; `ft4.Payload77` is an alias and inherits
its methods:

```go
func CallsignHash(call string, bits int) (int, bool)

type DXpeditionMessage struct {
    Call1, Call2  string
    SenderHash10 int
    Report       int
}

func (p Payload77) DXpedition() (DXpeditionMessage, bool)
```

`CallsignHash` accepts widths 10, 12, and 22 and uses the same public-input
normalization and validation as seeding. Invalid calls or widths return
`(0, false)`.

`DXpedition` returns `false` and a zero value for a different family, invalid
fields, or special/reserved recipient tokens (`n28 < ft8NTokens`). The legacy
text parser remains permissive for these token forms. Recipient calls use
stateless rendering, with `<...>` for a hashed
field. `SenderHash10` preserves the raw sender hash and `Report` is the numeric
dB report, from -30 through +32 in steps of two.

An application can compare a known station's hash with the sender field without
copying the bit layout or hash algorithm. A match means the call is a possible
sender: collisions, particularly with 10-bit hashes, prevent unique identity
proof from the hash alone.

## 4. Document decoder memory

Document the lifetime, learning fields, same-slot order, cap, collision policy,
seeding behavior, and concurrency limits for both decoder types. Distinguish
persistent decoder history from FT8's per-invocation temporary history and the
history-free FT4 package functions and payload parser.

The new API symbols are additive. The intended FT4 instance-rendering change is
`<...>` becoming `<CALL>` when the decoder's own history has a matching call.
No history is shared between instances or persisted outside them.

There is a behavior compatibility change: `ft4.Decoder` was stateless in
v0.9.0, and now retains mutable state. Integrators must create one instance per
receiver stream and call it sequentially. A mutex would serialize access but
would still mix different streams' histories. Use a new instance when changing
bands if history must start empty. Stateless package functions remain safe for
concurrent use. These migration requirements are included in
[the release notes](../CHANGELOG.md).
