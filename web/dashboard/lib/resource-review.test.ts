import { test } from 'node:test';
import assert from 'node:assert/strict';
import { resourceReviewActions } from './resource-review.ts';
void test('resource review permissions and transitions', () => {
  assert.deepEqual(
    resourceReviewActions('developer', 'administrator', 'submitted'),
    [],
  );
  assert.deepEqual(resourceReviewActions('admin', 'operator', 'submitted'), []);
  assert.deepEqual(resourceReviewActions('admin', 'reviewer', 'submitted'), [
    'approved',
    'rejected',
  ]);
  assert.deepEqual(resourceReviewActions('admin', 'reviewer', 'approved'), []);
  assert.deepEqual(
    resourceReviewActions('admin', 'administrator', 'approved'),
    ['signed', 'suspended'],
  );
  assert.deepEqual(resourceReviewActions('admin', 'administrator', 'signed'), [
    'suspended',
  ]);
  assert.deepEqual(
    resourceReviewActions('admin', 'administrator', 'rejected'),
    [],
  );
});
