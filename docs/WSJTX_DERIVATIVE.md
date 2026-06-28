# WSJT-X Derivative Status

This repository is a research FT8 decoder implementation derived from
WSJT-X/jt9 behavior and source-adjacent investigation.

Original contributions in this repository are copyright (C) 2026 Marc L. Veary (7Q5MLV).
That assertion does not claim ownership of preexisting WSJT-X/jt9
material or third-party components.

It is not a clean-room implementation and is not suitable for relicensing as a
permissive-license project. Treat this work as a GPLv3 WSJT-X derivative.

The purpose of this repository is pragmatic parity, decoder experimentation,
and as a dependency for [Station Manager](https://github.com/ColonelBlimp/station-manager):

- strict-mode parity with the installed `jt9 -8` oracle for the local WAV
  fixture corpus;
- deeper experimental decoding modes that may recover additional plausible
  messages;
- profiling and performance improvements for use inside a larger application.

Any redistribution of this repository or derivative binaries should preserve
the GPLv3 license text, this derivative-status notice, and applicable
third-party notices.
