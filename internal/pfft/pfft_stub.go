// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

//go:build !cgo || gonumfft

// Package pfft wraps the vendored PocketFFT C library, the default
// FFT backend for cgo-enabled builds.
//
// This stub keeps the package buildable when the PocketFFT backend is
// excluded — cgo disabled or the gonumfft build tag set. In those
// configurations the ft8 package selects the pure-Go Gonum backend and
// never imports pfft, and no C sources are compiled.
package pfft
