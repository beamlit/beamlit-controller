import { check } from 'k6';
import http from 'k6/http';

export const options = {
  stages: [
    { target: 5, duration: '1m' }, // low traffic
    { target: 20, duration: '1m' }, // ramp up to high traffic
    { target: 20, duration: '1m' }, // high traffic
    { target: 1, duration: '1m' }, // ramp down to low traffic
    { target: 1, duration: '1m' }, // low traffic
    { target: 0, duration: '1m' }, // no traffic
  ],
};

export default function () {
  const result = http.get('http://php-apache/');
  check(result, {
    'http response status code is 200': result.status === 200,
    'http response comes from beamlit': result.headers['X-Beamlit-Proxy'] === 'true',
  });
}