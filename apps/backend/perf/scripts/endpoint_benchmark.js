import http from 'k6/http';
import { check } from 'k6';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.4/index.js';

import { endpoints } from './endpoints.js';

const ENDPOINT = __ENV.ENDPOINT || 'get_me';
const RATE = parseInt(__ENV.RATE || '10', 10);
const DURATION = __ENV.DURATION || '30s';
const PRE_ALLOCATED_VUS = parseInt(__ENV.PRE_ALLOCATED_VUS || String(Math.max(RATE, 10)), 10);
const MAX_VUS = parseInt(__ENV.MAX_VUS || String(PRE_ALLOCATED_VUS * 2), 10);
const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:8081';
const K6_OUTPUT_DIR = __ENV.K6_OUTPUT_DIR;

export const options = {
  scenarios: {
    benchmark: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: PRE_ALLOCATED_VUS,
      maxVUs: MAX_VUS,
    },
  },
  thresholds: {
    'http_req_duration{expected_response:true}': ['p(95)<500'],
    'http_req_failed': ['rate<0.01'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

export default function () {
  const endpoint = endpoints[ENDPOINT];
  if (!endpoint) {
    throw new Error(`Unknown endpoint: ${ENDPOINT}`);
  }

  const ctx = {
    baseUrl: API_BASE_URL,
    vu: __VU,
    iteration: __ITER,
  };

  const req = endpoint.request(ctx);
  const params = {
    headers: req.headers || {},
    tags: { name: ENDPOINT },
  };

  const response = http.request(req.method, req.url, req.body || null, params);

  check(response, {
    'status is expected': (r) => endpoint.expectedStatuses.includes(r.status),
  });
}

export function handleSummary(data) {
  const result = {
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
  };

  if (K6_OUTPUT_DIR) {
    result[`${K6_OUTPUT_DIR}/summary.json`] = JSON.stringify(data, null, 2);
  }

  return result;
}
