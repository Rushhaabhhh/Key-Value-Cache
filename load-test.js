import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { group } from 'k6';

// Configuration
const KEY_POOL_SIZE = 100; // Significantly reduced from 1,000 to reduce cardinality
const VALUE_LENGTH = 256;
const PUT_RATIO = 0.5;  // 50% PUT requests
const BASE_URL = 'http://localhost:7171'; // Target the local server

// Test configuration
export const options = {
    stages: [
        { duration: '30s', target: 50 },    // Ramp up to 50 users
        { duration: '2m', target: 50 },     // Stay at 50 users for 2 minutes
        { duration: '30s', target: 100 },   // Ramp up to 100 users
        { duration: '2m', target: 100 },    // Stay at 100 users for 2 minutes
        { duration: '30s', target: 0 },     // Ramp down to 0 users
    ],
    thresholds: {
        http_req_duration: ['p(95)<500'], // 95% of requests should be below 500ms
    },
    // Reduce time series cardinality
    summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)'],
    discardResponseBodies: true,
    batchPerHost: 0
    , // Disable per-host batching
};

// Generate keys and values once at the start
// Using simple generator functions instead of imported randomString
function generateRandomString(length) {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    let result = '';
    for (let i = 0; i < length; i++) {
        result += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return result;
}

// Initialize pools and metrics
const keyPool = [];
const valuePool = [];

for (let i = 0; i < KEY_POOL_SIZE; i++) {
    keyPool.push(generateRandomString(8)); // Use smaller key size
    valuePool.push(generateRandomString(VALUE_LENGTH));
}

// Create custom metrics using k6's built-in Counter
const cacheHits = new Counter('cache_hits');
const cacheMisses = new Counter('cache_misses');
const totalGets = new Counter('total_gets');

export default function () {
    group('cache_operations', function () {
        // 50/50 GET/PUT ratio
        if (Math.random() < PUT_RATIO) {
            putRequest();
        } else {
            getRequest();
        }
    });
}

function putRequest() {
    const key = keyPool[Math.floor(Math.random() * keyPool.length)];
    const value = valuePool[Math.floor(Math.random() * valuePool.length)];

    const payload = JSON.stringify({ key: key, value: value });
    const params = {
        headers: {
            'Content-Type': 'application/json',
        },
        tags: { name: 'PutOperation' }, // Add a name tag to reduce unique time series
    };

    const response = http.post(`${BASE_URL}/put`, payload, params);

    check(response, {
        'put status is 200': (r) => r.status === 200,
    });
}

function getRequest() {
    const key = keyPool[Math.floor(Math.random() * keyPool.length)];

    // Separate parameters for cache hits and misses
    const hitParams = {
        tags: { name: 'GetOperation', result: 'hit' },
    };

    const missParams = {
        tags: { name: 'GetOperation', result: 'miss' },
    };

    // Start with default params for the request
    let params = {
        tags: { name: 'GetOperation' },
    };

    const response = http.get(`${BASE_URL}/get?key=${key}`, params);

    totalGets.add(1);

    if (response.status !== 200 || (response.body && (response.body.includes('error') || response.body.includes('ERROR')))) {
        cacheMisses.add(1, missParams.tags);
        check(response, {
            'get cache miss': (r) => true,
        });
    } else {
        cacheHits.add(1, hitParams.tags);
        check(response, {
            'get cache hit': (r) => true,
        });
    }

    check(response, {
        'get status is 200': (r) => r.status === 200,
    });
}