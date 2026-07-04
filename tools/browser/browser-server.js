// browser-server.js
// Playwright browser control server for toolset-api (Phase 4).
//
// Exposes a small HTTP API consumed by the Go gateway:
//   GET    /health                    -> liveness + session count
//   POST   /session                   -> create a session {browserType?}
//   GET    /session/:id               -> session metadata
//   DELETE /session/:id               -> destroy a session
//   POST   /session/:id/action        -> execute a DOM action
//
// Sessions are pooled per browser type and auto-closed after an idle timeout.
// Telemetry is disabled globally via container env vars (see Dockerfile).

const http = require('http');
const { chromium, firefox, webkit } = require('playwright');
const { v4: uuid } = require('uuid');

const PORT = parseInt(process.env.BROWSER_PORT || '3000', 10);

// Max concurrent sessions across all browser types (configurable, default 10).
const MAX_SESSIONS = parseInt(process.env.BROWSER_MAX_SESSIONS || '10', 10);

// Idle timeout before a session is auto-closed (minutes -> ms).
const SESSION_TIMEOUT_MINUTES = parseInt(
  process.env.BROWSER_SESSION_TIMEOUT_MINUTES || '30',
  10
);
const sessionTimeout = SESSION_TIMEOUT_MINUTES * 60 * 1000;

// Action-level default timeout (ms) for click/type/wait operations.
const DEFAULT_ACTION_TIMEOUT = parseInt(
  process.env.BROWSER_ACTION_TIMEOUT_MS || '5000',
  10
);

// Session store: id -> session.
const sessions = new Map();

// Browser pool: one long-lived browser per type, lazily launched.
const browserPool = {
  chromium: null,
  firefox: null,
  webkit: null,
};

// Helper: get or launch a browser of the given type.
async function getBrowser(type) {
  const key = type || 'chromium';
  if (!(key in browserPool)) {
    throw new Error(`Unsupported browser: ${key}`);
  }
  if (!browserPool[key]) {
    try {
      switch (key) {
        case 'chromium':
          browserPool[key] = await chromium.launch({ headless: true });
          break;
        case 'firefox':
          browserPool[key] = await firefox.launch({ headless: true });
          break;
        case 'webkit':
          browserPool[key] = await webkit.launch({ headless: true });
          break;
        default:
          throw new Error(`Unsupported browser: ${key}`);
      }
    } catch (err) {
      console.error(`Failed to launch ${key}:`, err);
      throw err;
    }
  }
  return browserPool[key];
}

// Helper: create a session, enforcing the global session cap.
async function createSession(browserType = 'chromium') {
  if (sessions.size >= MAX_SESSIONS) {
    const err = new Error(`session limit reached (${MAX_SESSIONS})`);
    err.code = 'SESSION_LIMIT';
    throw err;
  }

  const browser = await getBrowser(browserType);
  const context = await browser.newContext();
  const page = await context.newPage();

  const sessionId = uuid();
  const session = {
    id: sessionId,
    browserType: browserType || 'chromium',
    context,
    page,
    url: null,
    title: null,
    createdAt: new Date(),
    lastActivityAt: new Date(),
    timeout: null,
  };

  // Auto-close on idle.
  session.timeout = setTimeout(() => {
    console.log(`Closing idle session ${sessionId}`);
    closeSession(sessionId);
  }, sessionTimeout);

  sessions.set(sessionId, session);
  console.log(`Created session ${sessionId} (${session.browserType})`);
  return session;
}

// Helper: close and remove a session.
async function closeSession(sessionId) {
  const session = sessions.get(sessionId);
  if (!session) return;

  clearTimeout(session.timeout);

  try {
    await session.context.close();
  } catch (err) {
    console.error(`Error closing context ${sessionId}:`, err);
  }

  sessions.delete(sessionId);
  console.log(`Closed session ${sessionId}`);
}

// Helper: refresh a session's idle timer.
function refreshSessionTimeout(session) {
  clearTimeout(session.timeout);
  session.lastActivityAt = new Date();
  session.timeout = setTimeout(() => {
    console.log(`Closing idle session ${session.id}`);
    closeSession(session.id);
  }, sessionTimeout);
}

// Helper: execute a single DOM action against a session's page.
async function handleAction(session, action) {
  const { type, ...params } = action;
  refreshSessionTimeout(session);

  try {
    switch (type) {
      case 'navigate':
        await session.page.goto(params.url, {
          waitUntil: params.waitUntil || 'networkidle',
        });
        session.url = session.page.url();
        session.title = await session.page.title();
        return { status: 'success', url: session.url, title: session.title };

      case 'click':
        await session.page.click(params.selector, {
          timeout: params.timeout || DEFAULT_ACTION_TIMEOUT,
        });
        return { status: 'success', message: 'Clicked' };

      case 'type':
        await session.page.fill(params.selector, '');
        await session.page.type(params.selector, params.text, {
          delay: params.delay || 50,
        });
        return { status: 'success', message: 'Typed' };

      case 'eval': {
        const result = await session.page.evaluate(params.script, params.args);
        return { status: 'success', result };
      }

      case 'screenshot': {
        const buffer = await session.page.screenshot({
          fullPage: params.fullPage !== false,
        });
        return {
          status: 'success',
          data: buffer.toString('base64'),
          type: 'image/png',
        };
      }

      case 'pdf': {
        const pdfBuffer = await session.page.pdf({
          format: params.format || 'A4',
        });
        return {
          status: 'success',
          data: pdfBuffer.toString('base64'),
          type: 'application/pdf',
        };
      }

      case 'content': {
        const html = await session.page.content();
        return { status: 'success', html };
      }

      case 'wait_for_selector':
        await session.page.waitForSelector(params.selector, {
          timeout: params.timeout || DEFAULT_ACTION_TIMEOUT,
        });
        return { status: 'success', message: 'Selector appeared' };

      case 'wait_for_navigation':
        await session.page.waitForNavigation({
          timeout: params.timeout || DEFAULT_ACTION_TIMEOUT,
        });
        session.url = session.page.url();
        return { status: 'success', url: session.url };

      case 'get_title': {
        const title = await session.page.title();
        session.title = title;
        return { status: 'success', title };
      }

      case 'get_url': {
        const url = session.page.url();
        session.url = url;
        return { status: 'success', url };
      }

      case 'set_viewport':
        await session.page.setViewportSize({
          width: params.width,
          height: params.height,
        });
        return { status: 'success', message: 'Viewport set' };

      default:
        return { status: 'error', message: `Unknown action: ${type}` };
    }
  } catch (err) {
    console.error(`Action ${type} failed:`, err);
    return { status: 'error', message: err.message, error: err.toString() };
  }
}

// Helper: read and JSON-parse a request body.
function readBody(req) {
  return new Promise((resolve, reject) => {
    let body = '';
    req.on('data', (chunk) => {
      body += chunk.toString();
    });
    req.on('end', () => resolve(body));
    req.on('error', reject);
  });
}

// HTTP server.
const server = http.createServer(async (req, res) => {
  res.setHeader('Content-Type', 'application/json');

  // CORS (gateway is same-origin on the internal network; permissive here).
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    res.writeHead(200);
    res.end();
    return;
  }

  try {
    // GET /health
    if (req.method === 'GET' && req.url === '/health') {
      res.writeHead(200);
      res.end(
        JSON.stringify({
          status: 'ok',
          sessions: sessions.size,
          max_sessions: MAX_SESSIONS,
        })
      );
      return;
    }

    // POST /session
    if (req.method === 'POST' && req.url === '/session') {
      const body = await readBody(req);
      try {
        const { browserType } = JSON.parse(body || '{}');
        const session = await createSession(browserType);
        res.writeHead(201);
        res.end(
          JSON.stringify({
            id: session.id,
            browserType: session.browserType,
            createdAt: session.createdAt,
          })
        );
      } catch (err) {
        const status = err.code === 'SESSION_LIMIT' ? 429 : 500;
        res.writeHead(status);
        res.end(JSON.stringify({ error: err.message }));
      }
      return;
    }

    // DELETE /session/:id
    if (req.method === 'DELETE' && req.url.startsWith('/session/')) {
      const sessionId = req.url.split('/')[2];
      await closeSession(sessionId);
      res.writeHead(204);
      res.end();
      return;
    }

    // POST /session/:id/action
    if (
      req.method === 'POST' &&
      req.url.includes('/session/') &&
      req.url.includes('/action')
    ) {
      const sessionId = req.url.split('/')[2];
      const session = sessions.get(sessionId);
      if (!session) {
        res.writeHead(404);
        res.end(JSON.stringify({ error: 'Session not found' }));
        return;
      }
      const body = await readBody(req);
      try {
        const action = JSON.parse(body);
        const result = await handleAction(session, action);
        res.writeHead(200);
        res.end(JSON.stringify(result));
      } catch (err) {
        res.writeHead(400);
        res.end(JSON.stringify({ error: err.message }));
      }
      return;
    }

    // GET /session/:id
    if (req.method === 'GET' && req.url.startsWith('/session/')) {
      const sessionId = req.url.split('/')[2];
      const session = sessions.get(sessionId);
      if (!session) {
        res.writeHead(404);
        res.end(JSON.stringify({ error: 'Session not found' }));
        return;
      }
      res.writeHead(200);
      res.end(
        JSON.stringify({
          id: session.id,
          browserType: session.browserType,
          url: session.url,
          title: session.title,
          createdAt: session.createdAt,
          lastActivityAt: session.lastActivityAt,
        })
      );
      return;
    }

    // 404
    res.writeHead(404);
    res.end(JSON.stringify({ error: 'Not found' }));
  } catch (err) {
    console.error('Server error:', err);
    res.writeHead(500);
    res.end(JSON.stringify({ error: 'Internal server error' }));
  }
});

server.listen(PORT, '0.0.0.0', () => {
  console.log(`Browser server listening on port ${PORT}`);
  console.log(
    `max_sessions=${MAX_SESSIONS} idle_timeout=${SESSION_TIMEOUT_MINUTES}m`
  );
});

// Graceful shutdown: close all sessions + browsers, then exit.
process.on('SIGTERM', async () => {
  console.log('SIGTERM received, closing browsers...');
  for (const [sessionId] of sessions) {
    await closeSession(sessionId);
  }
  for (const [type, browser] of Object.entries(browserPool)) {
    if (browser) {
      await browser.close();
      console.log(`Closed ${type} browser`);
    }
  }
  server.close(() => {
    process.exit(0);
  });
});
