// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

//go:build pocketfft

/*
 * CGO compilation unit for PocketFFT.
 *
 * Go's cgo only compiles .c files in the package directory itself, not
 * recursively. The vendored upstream sits in pocketfft/ so the vendor
 * directory stays isolated and unmodified. This bridge file pulls the
 * upstream .c into the build via #include; the CFLAGS in pfft.go set
 * -I${SRCDIR}/pocketfft so the include resolves correctly.
 *
 * No package-side code lives here — pure inclusion.
 */
#include "pocketfft.c"
