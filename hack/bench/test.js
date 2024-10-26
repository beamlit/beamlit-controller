import { check } from 'k6';
import http from 'k6/http';

export const options = {
  stages: [
    { target: 5, duration: '1m' }, // low traffic
    { target: 10, duration: '1m' }, // ramp up to high traffic
    { target: 15, duration: '1m' }, // high traffic
    { target: 10, duration: '1m' }, // ramp down to low traffic
    { target: 5, duration: '1m' }, // low traffic
    { target: 0, duration: '1m' }, // no traffic
  ],
};

export default function () {
  const result = http.get('http://php-apache.beamlit/');
  check(result, {
    'http response status code is 200': result.status === 200,
    //'http response comes from beamlit': result.headers['cf-ray'] !== undefined,
  });
}