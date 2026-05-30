# Licensing

This repository contains a WSJT-X-derived FT8 decoder research implementation.
It is licensed under the GNU General Public License, version 3 only
(`GPL-3.0-only`).

The complete GPLv3 license text is in `LICENSE`.

## Copyright Notice

Original contributions in this repository are asserted under:

```text
Copyright (C) 2026 go-ft8 authors
```

Source files use SPDX headers where practical:

```text
SPDX-FileCopyrightText: 2026 go-ft8 authors
SPDX-License-Identifier: GPL-3.0-only
```

This notice applies only to original contributions in this repository.
Upstream WSJT-X/jt9 copyrights and third-party copyrights remain with their
respective owners.

## Scope

The GPLv3 license applies to this repository's FT8 decoder implementation,
tests, fixtures, reports, and project documentation unless a file or vendored
third-party directory states a different license.

This work is not a clean-room implementation. It was developed through
WSJT-X/jt9 parity research, source-adjacent investigation, and behavioral
comparison against an installed `jt9 -8` decoder. Treat the implementation as a
derivative work of WSJT-X for licensing purposes.

There is no permissive-license grant for this repository.

## Derivative Notice

WSJT-X and jt9 are GPL-licensed amateur-radio weak-signal communication tools.
This repository implements FT8 decoding behavior derived from that project and
therefore carries GPLv3 licensing forward.

See `WSJTX_DERIVATIVE.md` for the project-level derivative-status note.

## Third-Party Notices

The optional PocketFFT CGO backend under `internal/pfft/pocketfft/` is
vendored under the BSD 3-Clause license. Its license and provenance are
retained in:

- `internal/pfft/pocketfft/LICENSE.md`
- `internal/pfft/pocketfft/VENDORED.md`
