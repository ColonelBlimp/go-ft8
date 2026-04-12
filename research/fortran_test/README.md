# Fortran Reference Test Programs

These programs are **developer-only diagnostic tools** used to compare intermediate
values between the Go research pipeline and the original WSJT-X Fortran implementation.

## License Notice

These programs **link against WSJT-X Fortran source files** which are licensed under
**GPLv3** (see `~/Development/wsjt-wsjtx/COPYING`). They are NOT part of the go-ft8
library and are NOT distributed with it. They exist solely for local developer testing.

**Do not distribute these programs or their compiled binaries.**

The go-ft8 library itself contains only clean-room Go implementations of the same
algorithms. No GPLv3 code is included in the Go source files.

## Building

Requires `gfortran`, `libfftw3f-dev`, and the WSJT-X source tree at
`~/Development/wsjt-wsjtx/`. The Fortran programs include `ft8_params.f90`
from the WSJT-X source tree (not copied into this repository).

See the comments at the top of each `.f90` file for compile commands.

## Programs

- `dump_bmet.f90` — Dumps soft metrics for a single signal
- `dump_sync8.f90` — Dumps sync8 candidate list
- `dump_pass1.f90` — Runs full pass 1 decode loop (sync8 + ft8b, no subtraction)
- `dump_llr.f90` — Dumps LLR values and OSD decode results for a specific candidate
- `dump_osd_order.f90` — Dumps OSD reliability ordering
- `dump_osd_trace.f90` — Traces OSD internals (GE, order-0, order-1 search)
- `dump_gen.f90` — Dumps generator matrix rows
