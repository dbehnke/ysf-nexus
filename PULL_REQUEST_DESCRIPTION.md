PR: fix/openspot-status-response

Summary
-------
This branch contains fixes and features to improve compatibility with OpenSpot and similar YSF clients, plus improved logging for debugging.

Key changes
-----------
- network: accept minimal 4-byte YSFS probes and treat them as status requests
- network: produce space-padded poll/status responses matching pYSFReflector reference
- network: add side-by-side hex+ASCII hexdump output when debug logging is enabled
- network: concise INFO-level logging for YSF RX/TX when debug is disabled
- reflector: ensure status responses are produced and sent for minimal probes
- tests: add integration/e2e tests validating poll and status handshake

Why
---
OpenSpot and some other clients send a minimal 4-byte 'YSFS' probe; previously the server required a larger status request size and formatted status responses differently. These changes increase compatibility and make debugging the handshake much easier.

Notes
-----
- The hexdump is only printed when the server `debug` flag is true.
- INFO-level lines now include the actual YSF packet type (e.g., YSFP/YSFS) for RX and TX when debug is off.
- A new E2E test (`pkg/reflector/e2e_test.go`) runs the reflector in-process and validates the handshake.
