# syntax=docker/dockerfile:1

# Toolset SearXNG image.
# Wraps the upstream searxng/searxng image with our custom settings.yml and
# telemetry/metrics disabled. Runs internal-only on port 8888 (no host bind).
FROM searxng/searxng:latest

# Custom configuration. SEARXNG_SETTINGS_PATH tells SearXNG where to read it.
COPY settings.yml /etc/searxng/settings.yml

# Runtime environment. SEARXNG_SECRET is provided at run time by compose
# (openssl rand -hex 16); a build-time placeholder keeps the image self-contained.
ENV SEARXNG_SETTINGS_PATH=/etc/searxng/settings.yml \
    SEARXNG_PORT=8888 \
    SEARXNG_BIND_ADDRESS=0.0.0.0 \
    SEARXNG_SECRET=changeme \
    SEARXNG_AUTOCOMPLETE_MIN=0 \
    SEARXNG_ENABLE_METRICS=false

# Internal port only; do not publish to the host.
EXPOSE 8888

# Health check hits the SearXNG landing page.
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=20s \
    CMD curl -fsS http://localhost:8888/ >/dev/null || exit 1
