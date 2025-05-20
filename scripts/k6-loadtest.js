import http from 'k6/http';
import { sleep, check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

// Custom metrics
const submitScoreCounter = new Counter('submit_score_requests');
const getTopLeadersCounter = new Counter('get_top_leaders_requests');
const getPlayerRankCounter = new Counter('get_player_rank_requests');

const submitScoreErrors = new Rate('submit_score_errors');
const getTopLeadersErrors = new Rate('get_top_leaders_errors');
const getPlayerRankErrors = new Rate('get_player_rank_errors');

const submitScoreLatency = new Trend('submit_score_latency');
const getTopLeadersLatency = new Trend('get_top_leaders_latency');
const getPlayerRankLatency = new Trend('get_player_rank_latency');

// Configuration

const runTime = '5s';

export const options = {
  // scenarios: {
  //   health_check: {
  //     executor: 'constant-arrival-rate',
  //     rate: 1,
  //     timeUnit: '1s',
  //     duration: runTime,
  //     preAllocatedVUs: 1,
  //     maxVUs: 1,
  //     exec: 'healthCheck',
  //   },
  //   submit_scores: {
  //     executor: 'constant-arrival-rate',
  //     rate: 10000,
  //     timeUnit: '1s',
  //     duration: runTime,
  //     preAllocatedVUs: 1000,
  //     maxVUs: 2000,
  //     exec: 'submitScore',
  //   },
  //   get_top_leaders: {
  //     executor: 'constant-arrival-rate',
  //     rate: 2500,
  //     timeUnit: '1s',
  //     duration: runTime,
  //     preAllocatedVUs: 600,
  //     maxVUs: 800,
  //     exec: 'getTopLeaders',
  //   },
  //   get_player_rank: {
  //     executor: 'constant-arrival-rate',
  //     rate: 2500,
  //     timeUnit: '1s',
  //     duration: runTime,
  //     preAllocatedVUs: 600,
  //     maxVUs: 800,
  //     exec: 'getPlayerRank',
  //   },
  // },

  scenarios: {
    submit_scores: {
      executor: 'constant-arrival-rate',
      rate: 100,
      timeUnit: '1s',
      duration: runTime,
      preAllocatedVUs: 10,
      maxVUs: 20,
      exec: 'submitScore',
    },
    get_top_leaders: {
      executor: 'constant-arrival-rate',
      rate: 100,
      timeUnit: '1s',
      duration: runTime,
      preAllocatedVUs: 10,
      maxVUs: 20,
      exec: 'getTopLeaders',
    },
    get_player_rank: {
      executor: 'constant-arrival-rate',
      rate: 100,
      timeUnit: '1s',
      duration: runTime,
      preAllocatedVUs: 10,
      maxVUs: 20,
      exec: 'getPlayerRank',
    },
  },

  // thresholds: {
  //   'submit_score_latency{expected:true}': ['p(99)<50'],
  //   'get_top_leaders_latency{expected:true}': ['p(99)<50'],
  //   'get_player_rank_latency{expected:true}': ['p(99)<50'],
  
  //   'submit_score_requests': ['rate>=10000'],
  //   'get_top_leaders_requests': ['rate>=2500'],
  //   'get_player_rank_requests': ['rate>=2500'],
  // }
};

// Shared variables
// const BASE_URL = 'http://172.20.14.95:8080/api';
// const BASE_URL = 'http://172.20.14.95:80/api';
const BASE_URL = 'http://localhost:80/api';
const GAME_IDS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
const MAX_USER_ID = 10000000;


export function healthCheck() {
  const response = http.get(`${BASE_URL}/health`);
  check(response, {
    'status was OK': (r) => r.status == 200,
  });
}

// Submit Score function - write to the leaderboard
export function submitScore() {
  const gameId = GAME_IDS[randomIntBetween(0, GAME_IDS.length - 1)];
  const userId = randomIntBetween(1, MAX_USER_ID);
  const score = randomIntBetween(100, 10000);
  
  const payload = JSON.stringify({
    game_id: gameId,
    user_id: userId,
    score: score,
    timestamp: new Date().toISOString(),
  });
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };
  
  const startTime = new Date();
  const response = http.post(`${BASE_URL}/leaderboard/score`, payload, params);
  const endTime = new Date();
  
  submitScoreCounter.add(1);
  submitScoreLatency.add(endTime - startTime);
  
  const success = check(response, {
    'submit score status is 200': (r) => r.status === 200,
  });
  
  if (!success) {
    submitScoreErrors.add(1);
    console.log(`Submit score failed: ${response.status} ${response.body}`);
  }
  
  sleep(0.1);
}

// Get Top Leaders function - read from the leaderboard
export function getTopLeaders() {
  const gameId = GAME_IDS[randomIntBetween(0, GAME_IDS.length - 1)];
  const limit = randomIntBetween(10, 50);
  
  const startTime = new Date();
  const response = http.get(`${BASE_URL}/leaderboard/top/${gameId}?limit=${limit}`);
  const endTime = new Date();
  
  getTopLeadersCounter.add(1);
  getTopLeadersLatency.add(endTime - startTime);
  
  const success = check(response, {
    'get top leaders status is 200': (r) => r.status === 200,
    'get top leaders has leaders array': (r) => {
      try {
        const data = JSON.parse(r.body);
        return Array.isArray(data.leaders);
      } catch (e) {
        return false;
      }
    },
  });
  
  if (!success) {
    getTopLeadersErrors.add(1);
    console.log(`Get top leaders failed: ${response.status} ${response.body}`);
  }
  
  sleep(0.1);
}

// Get Player Rank function - read a player's rank
export function getPlayerRank() {
  const gameId = GAME_IDS[randomIntBetween(0, GAME_IDS.length - 1)];
  const userId = randomIntBetween(1, MAX_USER_ID);
  
  const startTime = new Date();
  const response = http.get(`${BASE_URL}/leaderboard/rank/${gameId}/${userId}`);
  const endTime = new Date();
  
  getPlayerRankCounter.add(1);
  getPlayerRankLatency.add(endTime - startTime);
  
  // We don't check for 404 errors when a player is not found
  // as that's an expected outcome
  const success = check(response, {
    'get player rank status is 200 or 404': (r) => r.status === 200 || r.status === 404,
  });
  
  if (!success) {
    getPlayerRankErrors.add(1);
    console.log(`Get player rank failed: ${response.status} ${response.body}`);
  }
  
  sleep(0.1);
} 


export function handleSummary(data) {
  const submitRPS = data.metrics['submit_score_requests'].rate;
  const topLeadersRPS = data.metrics['get_top_leaders_requests'].rate;
  const playerRankRPS = data.metrics['get_player_rank_requests'].rate;

  console.log('\n--- Custom RPS Report ---');
  console.log(`Submit Score RPS       : ${submitRPS.toFixed(2)} req/sec`);
  console.log(`Top Leaders RPS        : ${topLeadersRPS.toFixed(2)} req/sec`);
  console.log(`Player Rank RPS        : ${playerRankRPS.toFixed(2)} req/sec`);

  return {
    'summary.json': JSON.stringify(data, null, 2),
  };
}
