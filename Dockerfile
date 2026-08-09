# syntax=docker/dockerfile:1
# FLATLINE - autonomous CTF agent image (debuted at DEF CON 34 HALCTF).
# Multi-stage: (1) compile the Go harness + a couple of static Go offensive tools,
# (2) assemble a lean Debian runtime with the curated tool set and /skills.
# Target platform is linux/amd64 (the HALCTF GCE cluster). Build with:
#   podman build --platform linux/amd64 -t flatline .
#
# BuildKit/Buildah cache mounts are used on the download-heavy RUN steps (go mod,
# apt, pip) so re-builds reuse previously fetched artifacts instead of pulling
# them again. Cache-mount contents are NOT committed to the image, so they add
# zero size to the final tarball.

# ---------------------------------------------------------------------------
# Stage 1: build the harness binary + static Go tools.
# The agent compile command/flags are the mandated ones; only a module/build
# cache mount is added around it (does not change compilation semantics).
# ---------------------------------------------------------------------------
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /agent ./cmd/agent

# gobuster and ffuf are not in Debian main; they are pure-Go, so we build them
# here as small static binaries instead of pulling a heavier package source.
ENV GOBIN=/out
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go install -ldflags="-s -w" github.com/OJ/gobuster/v3@latest \
 && CGO_ENABLED=0 GOOS=linux go install -ldflags="-s -w" github.com/ffuf/ffuf/v2@latest

# nikto was dropped from Debian bookworm; it is a self-contained Perl script, so
# vendor it here (git is present in the golang image) and copy it into runtime.
RUN git clone --depth 1 https://github.com/sullo/nikto /opt/nikto

# enum4linux-ng is a single Python script (its PyPI name is squatted), so vendor
# it from GitHub too. Its deps (impacket, ldap3, pyyaml) are pip-installed at runtime.
RUN git clone --depth 1 https://github.com/cddmp/enum4linux-ng /opt/enum4linux-ng

# ---------------------------------------------------------------------------
# Stage 2: runtime. Curated offensive tool set + skills + the harness binary.
# ---------------------------------------------------------------------------
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    LANG=C.UTF-8 \
    TERM=xterm

# Package lists live in packages/ so the curated set is reviewable in one place.
COPY packages/apt-runtime.txt packages/pip-runtime.txt /tmp/

# apt tools (own layer, so it stays cached if the pip layer needs a rebuild).
# .debs and package lists live in cache mounts - reused across builds, never
# committed to the image. We still strip docs/man/locale from the real fs.
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    set -eux; \
    rm -f /etc/apt/apt.conf.d/docker-clean; \
    apt-get update; \
    apt-get install -y --no-install-recommends $(sed 's/#.*//' /tmp/apt-runtime.txt); \
    rm -rf /usr/share/doc /usr/share/man /usr/share/locale/*

# pip tools (own layer). Wheels live in a cache mount (not committed).
RUN --mount=type=cache,target=/root/.cache/pip \
    set -eux; \
    python3 -m pip install --break-system-packages $(sed 's/#.*//' /tmp/pip-runtime.txt); \
    rm -rf /tmp/*.txt; \
    find /usr/lib/python3* -type d -name __pycache__ -prune -exec rm -rf {} + 2>/dev/null || true

# Static Go tools built in stage 1, and vendored nikto/enum4linux-ng with PATH shims.
COPY --from=build /out/gobuster /out/ffuf /usr/local/bin/
COPY --from=build /opt/nikto /opt/nikto
COPY --from=build /opt/enum4linux-ng /opt/enum4linux-ng
RUN set -eux; \
    ln -sf /opt/nikto/program/nikto.pl /usr/local/bin/nikto; \
    chmod +x /opt/nikto/program/nikto.pl; \
    ln -sf /opt/enum4linux-ng/enum4linux-ng.py /usr/local/bin/enum4linux-ng; \
    chmod +x /opt/enum4linux-ng/enum4linux-ng.py; \
    # impacket examples install as GetNPUsers.py etc.; add Kali-style impacket-* aliases.
    for f in /usr/local/bin/*.py; do \
      [ -e "$f" ] || continue; \
      b=$(basename "$f" .py); \
      ln -sf "$f" "/usr/local/bin/impacket-$b"; \
    done

# Reusable playbooks (each dir has a SKILL.md; helper scripts are executable).
COPY skills/ /skills/

# The harness binary. It prints "USER ID: <uid>", heartbeats, reads all its
# env/endpoints itself - the entrypoint is just the binary, nothing else.
COPY --from=build /agent /usr/local/bin/agent
ENTRYPOINT ["/usr/local/bin/agent"]
