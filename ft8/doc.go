// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

// Package ft8 encodes and decodes FT8 messages.
//
// Production callers should normally create one Decoder per receiver stream and
// call (*Decoder).DecodeMessages once per 15-second FT8 slot. Decoder instances
// retain hash/history state and are not safe for concurrent use.
// DecodeMessagesWithReport and (*Decoder).DecodeMessagesWithReport return the
// same messages plus aggregate diagnostics for production observability.
// DecodeMessagesChecked and (*Decoder).DecodeMessagesChecked add strict input
// and option validation for service integrations.
//
// EncodeStandardMessage exposes the protocol encoder for supported standard
// FT8 messages. The package deliberately does not handle audio device output,
// transmit scheduling, PTT, or radio control.
//
// The decode pipeline deliberately keeps scale handling localized:
// candidate detection is ratio-based, symbol metrics are normalized before BP,
// and ft8ScaleFac sets the BP working LLR magnitude. Avoid changing scale
// constants unless the strict corpus parity test is used as a gate.
//
// License: this WSJT-X-derived go-ft8 implementation is distributed under
// GPL-3.0-only. See the repository LICENSE and docs/LICENSING.md.
package ft8
