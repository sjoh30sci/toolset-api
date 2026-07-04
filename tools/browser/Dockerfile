FROM mcr.microsoft.com/playwright:v1.40.0-jammy

# --- Telemetry / CI hardening -------------------------------------------------
# Disable interactive debug + noisy telemetry. Browsers ship in the base image
# so we do NOT skip the download (0), but we pin CI mode for deterministic runs.
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=0
ENV PWDEBUG=0
ENV CI=true
ENV FIREFOX_TMP_DIR=/tmp/firefox
ENV WEBKIT_LAUNCH_OPTIONS=""
ENV NODE_ENV=production

# --- Additional dependencies --------------------------------------------------
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# --- Non-root user ------------------------------------------------------------
RUN useradd -m -s /bin/bash playwright \
    && mkdir -p /tmp/firefox \
    && chown -R playwright:playwright /tmp/firefox

WORKDIR /app

# Install production dependencies first for better layer caching.
COPY package.json ./
RUN npm install --omit=dev --no-audit --no-fund

# Copy the server implementation.
COPY browser-server.js ./browser-server.js

# Expose the HTTP control port (internal only in compose).
EXPOSE 3000

# Health check hits the /health endpoint.
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:3000/health || exit 1

# Run as the restricted playwright user (has access to bundled browsers).
USER playwright
ENTRYPOINT ["node", "/app/browser-server.js"]
