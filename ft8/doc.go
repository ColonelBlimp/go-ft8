// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

// Package ft8 decodes FT8 messages from 12 kHz mono signed-16-bit PCM audio.
//
// Production callers should normally create one Decoder per receiver stream and
// call (*Decoder).DecodeMessages once per 15-second FT8 slot. Decoder instances
// retain hash/history state and are not safe for concurrent use.
//
// The decode pipeline deliberately keeps scale handling localized:
// candidate detection is ratio-based, symbol metrics are normalized before BP,
// and ft8ScaleFac sets the BP working LLR magnitude. Avoid changing scale
// constants unless the strict corpus parity test is used as a gate.
//
// License: this WSJT-X-derived go-ft8 implementation is distributed under
// GPL-3.0-only. See the repository LICENSE and docs/LICENSING.md.
package ft8
