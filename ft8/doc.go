// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

// Package ft8 encodes and decodes FT8 messages.
//
// Production callers should normally create one Decoder per receiver stream and
// call (*Decoder).DecodeMessages once per 15-second FT8 slot. Decoder instances
// retain hash/history state and are not safe for concurrent use.
// DecodeMessagesWithReport and (*Decoder).DecodeMessagesWithReport return the
// same messages plus aggregate diagnostics for production observability,
// including AP profile/source attempt, success, and post-LDPC rejection
// counters.
// DecodeMessagesChecked and (*Decoder).DecodeMessagesChecked add strict input
// and option validation for service integrations.
// DecodeStructuredWithReport and DecodeStructuredChecked provide the same
// diagnostics and validation for strict/deep labeled structured output.
// Checked decode errors support errors.Is with ErrInvalidDecodeInput and
// ErrInvalidDecoderOptions, and errors.As with DecodeInputError or
// DecoderOptionError for structured validation details.
//
// Decode supports free text, DXpedition/Fox-Hound, telemetry, standard /R and
// /P messages, ARRL Field Day, ARRL RTTY Roundup, type 4
// compound/nonstandard-call messages, and type 5 EU VHF contest exchanges.
// Every signal-level candidate accepted after LDPC, CRC, hard-error, and
// all-zero checks is returned with its verified Payload77, ParseStatus, and
// DecodeProvenance. Assigned-but-unsupported, reserved, and structurally
// invalid payloads are returned without canonical text. The package does not
// suppress /R or TU; observations; publication filtering belongs to callers.
// The default decoder includes a conservative CQ AP profile. EnableBroadAP and
// DeepDecoderOptions add experimental directed-CQ AP profiles.
// APCallHints and (*Decoder).SetAPCallHints provide upstream-ranked callsign
// hints for bounded BP-only AP; the decoder copies, normalizes, deduplicates,
// caps, cheaply scores per candidate, and tries only the top hypotheses.
// ARRLFieldDaySections, ParseARRLFieldDaySection, and
// ValidARRLFieldDaySection expose the ARRL/RAC Field Day section contract used
// by the encoder.
//
// EncodeStandardMessage exposes the protocol encoder for supported standard
// FT8 messages, including /P and /R suffix forms, ARRL Field Day exchanges, and
// type 4 compound/nonstandard calls. Type 4 follows WSJT-X pack77_4: the CQ
// form accepts any nonstandard call of 5-11 base-38 characters, and in the
// two-call form a <bracketed> call is sent as a 12-bit hash while the other
// call is spelled verbatim, preserving affixes such as /P. Unbracketed
// compound+standard pairs deviate deliberately from WSJT-X by keeping the
// full compound call (WSJT-X drops the prefix), and inputs WSJT-X would
// encode lossily are rejected. The package deliberately does not handle
// audio device output, transmit scheduling, PTT, or radio control.
//
// The decode pipeline deliberately keeps scale handling localized:
// candidate detection is ratio-based, symbol metrics are normalized before BP,
// and ft8ScaleFac sets the BP working LLR magnitude. Avoid changing scale
// constants unless the strict corpus parity test is used as a gate.
//
// License: this WSJT-X-derived go-ft8 implementation is distributed under
// GPL-3.0-only. See the repository LICENSE and docs/LICENSING.md.
package ft8
