# Vendored PocketFFT

This directory contains a verbatim copy of upstream PocketFFT, vendored
into this research tree for reproducible CGO builds.

## Source

- **Upstream:** <https://gitlab.mpcdf.mpg.de/mtr/pocketfft>
- **Commit:** `81d171a6d5562e3aaa2c73489b70f564c633ff81`
- **Date:** 2019-05-10
- **Vendored on:** 2026-05-27

## Files included

- `pocketfft.c` — implementation (62 KB, verbatim from upstream)
- `pocketfft.h` — public C API (verbatim from upstream)
- `LICENSE.md` — BSD 3-Clause licence text (Copyright Max-Planck-Society 2010-2019)
- `README.md` — upstream README (algorithmic notes, references)

## Files NOT included

- `ffttest.c`, `TESTING` — upstream test harness (we run our own tests via Go)
- `.gitlab-ci.yml`, `.git/` — upstream build/version metadata

## Modifications

None. The vendored files are byte-identical to the upstream commit. If
local patches ever become necessary they will be applied as separate
`.patch` files in this directory with a clear comment in this file,
never inline edits.

## Licence

PocketFFT is BSD 3-Clause (see `LICENSE.md`). Attribution requirement
is honoured by retaining the copyright notice in this directory; binary
distributions that link against this code must reproduce the copyright
notice in their distribution materials.
