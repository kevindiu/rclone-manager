import assert from 'node:assert/strict';
import test from 'node:test';
import {
  buildUploadQueueFromFileList,
  partitionUploadBatches,
  resolveUploadDir,
} from '../src/utils/upload.ts';

test('resolveUploadDir handles root, nested paths, Windows separators, and traversal segments', () => {
  assert.equal(resolveUploadDir('', ''), '');
  assert.equal(resolveUploadDir('', 'report.pdf'), '');
  assert.equal(resolveUploadDir('backups', 'report.pdf'), 'backups');
  assert.equal(resolveUploadDir('', 'photos/2026/a.jpg'), 'photos/2026');
  assert.equal(resolveUploadDir('backups/work', 'photos/2026/a.jpg'), 'backups/work/photos/2026');
  assert.equal(resolveUploadDir('/backups/', 'sub\\nested\\a.txt'), 'backups/sub/nested');
  assert.equal(resolveUploadDir('backups', '../secret/../../evil.txt'), 'backups/secret');
});

test('buildUploadQueueFromFileList preserves webkitRelativePath and plain filenames', () => {
  const plainFile = new File(['hello'], 'notes.txt', { type: 'text/plain' });
  const nestedFile = new File(['img'], 'photo.png', { type: 'image/png' });
  Object.defineProperty(nestedFile, 'webkitRelativePath', {
    value: 'trip/day1/photo.png',
    configurable: true,
  });

  const queue = buildUploadQueueFromFileList([plainFile, nestedFile], 'archive');
  assert.equal(queue.length, 2);
  assert.equal(queue[0]?.dirPath, 'archive');
  assert.equal(queue[0]?.relPath, 'notes.txt');
  assert.equal(queue[0]?.displayPath, 'notes.txt');
  assert.equal(queue[1]?.dirPath, 'archive/trip/day1');
  assert.equal(queue[1]?.relPath, 'trip/day1/photo.png');
  assert.equal(queue[1]?.displayPath, 'trip/day1/photo.png');
});

test('partitionUploadBatches packs small files and keeps solo/large files as single streams', () => {
  const makeSizedFile = (name: string, size: number) => {
    const f = new File([new Uint8Array(size)], name);
    return f;
  };

  // Single file stays single
  const solo = buildUploadQueueFromFileList([makeSizedFile('one.txt', 10)], '');
  const soloPlan = partitionUploadBatches(solo);
  assert.equal(soloPlan.length, 1);
  assert.equal(soloPlan[0]?.kind, 'single');

  // Small files pack into batches respecting maxFiles & maxBytes; large file splits to single
  const items = buildUploadQueueFromFileList(
    [
      makeSizedFile('s1.txt', 100),
      makeSizedFile('s2.txt', 100),
      makeSizedFile('s3.txt', 100),
      makeSizedFile('big.bin', 500),
    ],
    'docs'
  );
  const plan = partitionUploadBatches(items, {
    smallFileMaxBytes: 250,
    maxFiles: 2,
    maxBytes: 1000,
  });
  assert.equal(plan.length, 3);
  assert.equal(plan[0]?.kind, 'batch');
  if (plan[0]?.kind === 'batch') assert.equal(plan[0].items.length, 2);
  assert.equal(plan[1]?.kind, 'single');
  if (plan[1]?.kind === 'single') assert.equal(plan[1].item.relPath, 's3.txt');
  assert.equal(plan[2]?.kind, 'single');
  if (plan[2]?.kind === 'single') assert.equal(plan[2].item.relPath, 'big.bin');
});
