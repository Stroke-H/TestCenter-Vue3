import http from 'k6/http';
import { check, sleep } from 'k6';
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";
import { textSummary } from "https://jslib.k6.io/k6-summary/0.0.1/index.js";

export const options = {
  // A simple 1-minute ramp up and down
  stages: [
    { duration: '10s', target: 20 }, // string up to 20 users
    { duration: '30s', target: 20 }, // stay at 20 peers
    { duration: '10s', target: 0 },  // scale down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests must complete below 500ms
  },
};

export default function () {
  // Replace with actual episode playback API endpoint
  const url = 'http://localhost:3000/api/mock/episode/play';

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer YOUR_MOCK_TOKEN'
    },
  };

  const payload = JSON.stringify({
    drama_id: '12345',
    episode_index: 1,
    user_id: 'test_user_' + __VU // Virtual User ID
  });

  const res = http.post(url, payload, params);

  check(res, {
    'is status 200': (r) => r.status === 200,
    'has playback url': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body && body.data && body.data.play_url !== '';
      } catch (e) {
        return false;
      }
    }
  });

  // User reading/waiting time simulation
  sleep(1);
}

// Function executed at the end of the test to generate the HTML report
export function handleSummary(data) {
  return {
    "reports/summary.html": htmlReport(data, { title: "Episode Performance Test (剧集播放压测报告)" }),
    stdout: textSummary(data, { indent: " ", enableColors: false }),
  };
}
