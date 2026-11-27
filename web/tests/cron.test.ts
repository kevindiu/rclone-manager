import assert from 'node:assert/strict';
import test from 'node:test';
import {
  buildCronExpression,
  describeCronSchedule,
  parseCronSchedule,
} from '../src/utils/cron.ts';

test('parseCronSchedule and buildCronExpression round-trip common modes', () => {
  assert.equal(parseCronSchedule('').mode, 'manual');
  assert.equal(buildCronExpression(parseCronSchedule('')), '');

  const every15m = parseCronSchedule('*/15 * * * *');
  assert.equal(every15m.mode, 'interval');
  assert.equal(every15m.intervalUnit, 'minutes');
  assert.equal(every15m.intervalEvery, 15);
  assert.equal(buildCronExpression(every15m), '*/15 * * * *');

  const hourly = parseCronSchedule('30 * * * *');
  assert.equal(hourly.mode, 'interval');
  assert.equal(hourly.intervalUnit, 'hours');
  assert.equal(hourly.intervalEvery, 1);
  assert.equal(hourly.minute, 30);
  assert.equal(buildCronExpression(hourly), '30 * * * *');

  const every6h = parseCronSchedule('0 */6 * * *');
  assert.equal(every6h.mode, 'interval');
  assert.equal(every6h.intervalEvery, 6);
  assert.equal(buildCronExpression(every6h), '0 */6 * * *');

  const daily = parseCronSchedule('15 2 * * *');
  assert.equal(daily.mode, 'daily');
  assert.equal(daily.hour, 2);
  assert.equal(daily.minute, 15);
  assert.equal(buildCronExpression(daily), '15 2 * * *');

  const weekdays = parseCronSchedule('0 3 * * 1-5');
  assert.equal(weekdays.mode, 'weekly');
  assert.deepEqual(weekdays.weekdays, [1, 2, 3, 4, 5]);
  assert.equal(buildCronExpression(weekdays), '0 3 * * 1-5');

  const weekendWith7 = parseCronSchedule('45 23 * * 6,7');
  assert.equal(weekendWith7.mode, 'weekly');
  assert.deepEqual(weekendWith7.weekdays, [0, 6]);
  assert.equal(buildCronExpression(weekendWith7), '45 23 * * 0,6');

  const monthly = parseCronSchedule('0 4 21 * *');
  assert.equal(monthly.mode, 'monthly');
  assert.equal(monthly.dayOfMonth, 21);
  assert.equal(monthly.hour, 4);
  assert.equal(buildCronExpression(monthly), '0 4 21 * *');
});

test('parseCronSchedule handles macros and falls back to custom for complex cron', () => {
  assert.equal(parseCronSchedule('@hourly').mode, 'interval');
  assert.equal(parseCronSchedule('@daily').mode, 'daily');
  assert.equal(parseCronSchedule('@weekly').mode, 'weekly');
  assert.equal(parseCronSchedule('@monthly').mode, 'monthly');

  const complex = parseCronSchedule('0 2 * 1,6 *');
  assert.equal(complex.mode, 'custom');
  assert.equal(complex.customExpr, '0 2 * 1,6 *');
});

test('describeCronSchedule formats plain-English summaries', () => {
  assert.equal(describeCronSchedule(''), 'Manual only');
  assert.equal(describeCronSchedule('*/15 * * * *'), 'Every 15 minutes');
  assert.equal(describeCronSchedule('0 * * * *'), 'Every hour');
  assert.equal(describeCronSchedule('15 */6 * * *'), 'Every 6 hours at :15');
  assert.equal(describeCronSchedule('0 2 * * *'), 'Daily at 02:00');
  assert.equal(describeCronSchedule('30 9 * * 1-5'), 'Weekdays (Mon–Fri) at 09:30');
  assert.equal(describeCronSchedule('0 10 * * 0,6'), 'Weekends (Sat–Sun) at 10:00');
  assert.equal(describeCronSchedule('0 8 * * 1,3,5'), 'Every Mon, Wed, Fri at 08:00');
  assert.equal(describeCronSchedule('0 4 1 * *'), 'Monthly on the 1st at 04:00');
  assert.equal(describeCronSchedule('0 4 12 * *'), 'Monthly on the 12th at 04:00');
  assert.equal(describeCronSchedule('0 4 22 * *'), 'Monthly on the 22nd at 04:00');
  assert.equal(describeCronSchedule('0 2 * 1 *'), 'Custom: 0 2 * 1 *');
});
