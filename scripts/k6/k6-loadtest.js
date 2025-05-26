import http from 'k6/http';
import { sleep, check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

const BASE_URL = 'http://localhost:8080/api';

// 📊 Custom metrics
const submitScoreCounter = new Counter('submit_score_requests');
const getTopLeadersCounter = new Counter('get_top_leaders_requests');
const getPlayerRankCounter = new Counter('get_player_rank_requests');

const submitScoreLatency = new Trend('submit_score_latency');
const getTopLeadersLatency = new Trend('get_top_leaders_latency');
const getPlayerRankLatency = new Trend('get_player_rank_latency');

const submitScoreErrors = new Rate('submit_score_errors');
const getTopLeadersErrors = new Rate('get_top_leaders_errors');
const getPlayerRankErrors = new Rate('get_player_rank_errors');

const writeRate = __ENV.WRITE_RATE ? parseInt(__ENV.WRITE_RATE) : 15000; // 4k writes/s
const readRate = __ENV.READ_RATE ? parseInt(__ENV.READ_RATE) : 10000;   // 3k reads/s per scenario
const timeUnit = '1s';
const runTime = '10s';

// To achieve this load, you need a lot of VUs preallocated and max
const preAllocatedVUs = 1000;
const maxVUs = 2000;

export const options = {
  scenarios: {
    health_check: {
      executor: 'constant-arrival-rate',
      rate: 10,
      timeUnit: '2s',
      duration: '30s',
      preAllocatedVUs: 10,
      maxVUs: 10,
      exec: 'healthCheck',
    },
    submit_scores: {
      executor: 'constant-arrival-rate',
      rate: writeRate,
      timeUnit: timeUnit,
      duration: runTime,
      preAllocatedVUs: preAllocatedVUs,
      maxVUs: maxVUs,
      exec: 'submitScore',
    },
    get_top_leaders: {
      executor: 'constant-arrival-rate',
      rate: readRate,
      timeUnit: timeUnit,
      duration: runTime,
      preAllocatedVUs: preAllocatedVUs,
      maxVUs: maxVUs,
      exec: 'getTopLeaders',
    },
    get_player_rank: {
      executor: 'constant-arrival-rate',
      rate: readRate,
      timeUnit: timeUnit,
      duration: runTime,
      preAllocatedVUs: preAllocatedVUs,
      maxVUs: maxVUs,
      exec: 'getPlayerRank',
    },
  },
};

// 🔗 Base URL and game/user data
const GAME_IDS = Array.from({ length: 50 }, (_, i) => i + 1);
const MAX_USER_ID = 1_000_000_000;

// 🩺 Health Check
export function healthCheck() {
  const res = http.get(`${BASE_URL}/health`);
  check(res, { 'status was 200': (r) => r.status === 200 });
}

// 📝 Submit Score
export function submitScore() {
  const gameId = GAME_IDS[randomIntBetween(0, GAME_IDS.length - 1)];
  const userId = randomIntBetween(1, MAX_USER_ID);
  const score = randomIntBetween(100, 1_000_000);

  const payload = JSON.stringify({
    game_id: gameId,
    user_id: userId,
    score,
    timestamp: new Date().toISOString(),
  });

  const params = { headers: { 'Content-Type': 'application/json' } };
  const start = Date.now();
  const res = http.post(`${BASE_URL}/leaderboard/score`, payload, params);
  const latency = Date.now() - start;

  submitScoreCounter.add(1);
  submitScoreLatency.add(latency);
  const ok = check(res, {
    'submit score status 200': (r) => r.status === 200,
  });
  submitScoreErrors.add(!ok);
  sleep(0.1);
}

// 📈 Get Top Leaders
export function getTopLeaders() {
  const gameId = GAME_IDS[randomIntBetween(0, GAME_IDS.length - 1)];
  const limit = randomIntBetween(10, 50);

  const start = Date.now();
  const res = http.get(`${BASE_URL}/leaderboard/top/${gameId}?limit=${limit}`);
  const latency = Date.now() - start;

  getTopLeadersCounter.add(1);
  getTopLeadersLatency.add(latency);

  const ok = check(res, {
    'top leaders status 200': (r) => r.status === 200,
    'top leaders has leaders[]': (r) => {
      try {
        const data = JSON.parse(r.body);
        return Array.isArray(data.leaders);
      } catch (_) {
        return false;
      }
    },
  });
  getTopLeadersErrors.add(!ok);
  sleep(0.1);
}

// 🏅 Get Player Rank
export function getPlayerRank() {
  const gameId = GAME_IDS[randomIntBetween(0, GAME_IDS.length - 1)];
  const userId = randomIntBetween(1, MAX_USER_ID);

  const start = Date.now();
  const res = http.get(`${BASE_URL}/leaderboard/rank/${gameId}/${userId}`);
  const latency = Date.now() - start;

  getPlayerRankCounter.add(1);
  getPlayerRankLatency.add(latency);

  const ok = check(res, {
    'player rank 200 or 404': (r) => r.status === 200 || r.status === 404,
  });
  getPlayerRankErrors.add(!ok);
  sleep(0.1);
}
