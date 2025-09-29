## TL;DR

A Go-based YSF reflector with OpenSpot compatibility, a web dashboard, MQTT events, and bridge scaffolding. Production-ready core, tests and CI included.


## Quick Summary

- UDP-based packet handling for YSFP/YSFU/YSFD/YSFS
- OpenSpot-compatible status handling (accepts 4-byte YSFS probes)
- Single-active-stream enforcement and talk-timeout muting
- Embedded web dashboard with WebSocket updates
- MQTT event publishing for external integrations
- Bridge framework for scheduled inter-reflector connections


## Concise Checklist

- [x] Build the binary: `make build`
- [x] Run with default config: `./bin/ysf-nexus`
- [x] Run tests: `make test`
- [x] Confirm OpenSpot compatibility: e2e reflector test (accepts 4-byte YSFS probe)
- [ ] Tune talk timeouts via config (`server.talk_max_duration` / `server.unmute_after`)
- [ ] (Optional) Enable MQTT and web dashboard in `config.yaml`


---

This PR contains a set of fixes and improvements focused on OpenSpot status probe compatibility, consolidated logging, and repeater arbitration (single-active-stream + muting). See individual commits for details and the `COPILOT.md` / `CLAUDE.md` files for a short project summary and implementation status.
