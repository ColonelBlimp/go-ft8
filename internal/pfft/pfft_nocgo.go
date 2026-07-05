// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

//go:build !cgo

// Package pfft wraps the vendored PocketFFT C library, the default
// FFT backend for cgo-enabled builds.
//
// This stub keeps the package buildable when cgo is disabled; the ft8
// package selects the pure-Go Gonum backend instead and never imports
// pfft in that configuration.
package pfft
