import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const layout = readFileSync(new URL('../src/layouts/Layout.astro', import.meta.url), 'utf8');
const body = layout.match(/before_send: function \(event\) \{([\s\S]*?)\n          \},/)[1];
const beforeSend = vm.runInNewContext(`(function(event) {${body}})`);
const event = (properties) => ({ event: '$exception', properties });
for (const properties of [
  { $exception_list: [{ type: 'AbortError', value: 'PostHog request timed out after 3000ms' }] },
  { $exception_list: [{ $exception_type: 'AbortError', $exception_value: 'PostHog request timed out after 3000ms' }] },
  { $exception_types: ['AbortError'], $exception_values: ['PostHog request timed out after 3000ms'] },
]) assert.equal(beforeSend(event(properties)), null);
for (const properties of [
  { $exception_list: [{ type: 'AbortError', value: 'signal is aborted without reason' }] },
  { $exception_list: [{ type: 'Error', value: 'PostHog request timed out after 3000ms' }] },
  { $exception_types: ['Error', 'AbortError'], $exception_values: ['PostHog request timed out after 3000ms', 'app abort'] },
  { $exception_values: ['PostHog request timed out after 3000ms'] },
  { $exception_list: [{ type: 'SyntaxError', value: 'Unexpected token ?' }] },
  { $exception_list: [{ type: 'APIError', value: 'Network Error' }] },
]) { const input = event(properties); assert.equal(beforeSend(input), input); }
const analytics = { event: '$pageview', properties: {} };
assert.equal(beforeSend(analytics), analytics);
console.log('Marketing exception filtering regressions passed');
