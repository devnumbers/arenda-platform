const REPORT_ENDPOINT = "/api/client-errors";
const REPORT_APP = "landing";
const MAX_REPORTS_PER_MINUTE = 5;

let initialized = false;
let lastMessage: string | undefined;
let sentCount = 0;
let windowStartedAt = 0;

interface ClientErrorPayload {
  app: string;
  message: string;
  stack?: string;
  url?: string;
}

function send(message: string, stack?: string): void {
  const now = Date.now();
  if (now - windowStartedAt >= 60_000) {
    windowStartedAt = now;
    sentCount = 0;
  }
  if (sentCount >= MAX_REPORTS_PER_MINUTE) {
    return;
  }
  sentCount += 1;

  const payload: ClientErrorPayload = {
    app: REPORT_APP,
    message,
    url: window.location.href,
  };
  if (stack) {
    payload.stack = stack;
  }

  fetch(REPORT_ENDPOINT, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    keepalive: true,
  }).catch(() => {
    // Reporting must never surface its own failures.
  });
}

function reportClientError(message: string, stack?: string): void {
  try {
    if (!message || message === lastMessage) {
      return;
    }
    lastMessage = message;
    send(message, stack);
  } catch {
    // Reporting must never break the app.
  }
}

function onError(event: ErrorEvent): void {
  const stack = event.error instanceof Error ? event.error.stack : undefined;
  reportClientError(event.message, stack);
}

function onUnhandledRejection(event: PromiseRejectionEvent): void {
  const reason: unknown = event.reason;
  if (typeof reason === "string") {
    reportClientError(reason);
  } else if (reason instanceof Error) {
    reportClientError(reason.message, reason.stack);
  } else {
    reportClientError(String(reason));
  }
}

/** Subscribe to global browser errors and POST them to the backend. Idempotent. */
export function initErrorReporting(): void {
  if (initialized) {
    return;
  }
  // The landing dev server has no /api proxy — /api exists only where the backend is same-origin.
  if (location.hostname === "localhost" || location.hostname === "127.0.0.1") {
    return;
  }
  initialized = true;
  window.addEventListener("error", onError);
  window.addEventListener("unhandledrejection", onUnhandledRejection);
}
