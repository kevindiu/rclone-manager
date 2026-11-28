export interface QueuedUpload {
  file: File;
  dirPath: string;
  relPath: string;
  displayPath: string;
}

export const BATCH_SMALL_FILE_MAX_BYTES = 16 * 1024 * 1024;
export const BATCH_MAX_FILES = 80;
export const BATCH_MAX_BYTES = 64 * 1024 * 1024;

export type UploadWorkItem =
  | { kind: 'single'; item: QueuedUpload }
  | { kind: 'batch'; items: QueuedUpload[] };

interface DropFileSystemEntry {
  isFile: boolean;
  isDirectory: boolean;
  name: string;
  fullPath?: string;
}

interface DropFileEntry extends DropFileSystemEntry {
  isFile: true;
  file: (success: (file: File) => void, error?: (err: DOMException) => void) => void;
}

interface DropDirectoryReader {
  readEntries: (
    success: (entries: DropFileSystemEntry[]) => void,
    error?: (err: DOMException) => void
  ) => void;
}

interface DropDirectoryEntry extends DropFileSystemEntry {
  isDirectory: true;
  createReader: () => DropDirectoryReader;
}

function cleanPathSegments(rawPath: string): string[] {
  return rawPath
    .replaceAll('\\', '/')
    .split('/')
    .filter((seg) => seg !== '' && seg !== '.' && seg !== '..');
}

/**
 * Joins baseSubpath and the parent directory segments of relativeFilePath.
 */
export function resolveUploadDir(baseSubpath: string, relativeFilePath?: string): string {
  const baseSegments = cleanPathSegments(baseSubpath);
  const relSegments = cleanPathSegments(relativeFilePath ?? '');
  const relDirSegments = relSegments.length > 1 ? relSegments.slice(0, -1) : [];
  return [...baseSegments, ...relDirSegments].join('/');
}

/**
 * Converts a FileList (from single/multi-file or webkitdirectory input) into a queued upload list.
 */
export function buildUploadQueueFromFileList(
  files: FileList | File[],
  baseSubpath: string
): QueuedUpload[] {
  return Array.from(files).map((file) => {
    const webkitRel = file.webkitRelativePath || '';
    const relPath = cleanPathSegments(webkitRel || file.name).join('/') || file.name;
    return {
      file,
      dirPath: resolveUploadDir(baseSubpath, webkitRel),
      relPath,
      displayPath: webkitRel || file.name,
    };
  });
}

function readAllDirectoryEntries(reader: DropDirectoryReader): Promise<DropFileSystemEntry[]> {
  return new Promise((resolve, reject) => {
    const entries: DropFileSystemEntry[] = [];
    const readNext = () => {
      reader.readEntries(
        (batch) => {
          if (batch.length === 0) {
            resolve(entries);
            return;
          }
          entries.push(...batch);
          readNext();
        },
        (err) => reject(err)
      );
    };
    readNext();
  });
}

function readEntryFile(entry: DropFileEntry): Promise<File> {
  return new Promise((resolve, reject) => {
    entry.file(resolve, reject);
  });
}

async function traverseDropEntry(
  entry: DropFileSystemEntry,
  parentRelSegments: string[],
  baseSubpath: string,
  files: QueuedUpload[],
  emptyDirs: string[]
): Promise<void> {
  const seg = entry.name.trim();
  if (!seg || seg === '.' || seg === '..') {
    return;
  }
  const currentSegments = [...parentRelSegments, seg];

  if (entry.isFile) {
    const file = await readEntryFile(entry as DropFileEntry);
    const displayPath = currentSegments.join('/');
    files.push({
      file,
      dirPath: resolveUploadDir(baseSubpath, displayPath),
      relPath: displayPath,
      displayPath,
    });
    return;
  }

  if (entry.isDirectory) {
    const dirEntry = entry as DropDirectoryEntry;
    const children = await readAllDirectoryEntries(dirEntry.createReader());
    if (children.length === 0) {
      emptyDirs.push([...cleanPathSegments(baseSubpath), ...currentSegments].join('/'));
      return;
    }
    for (const child of children) {
      await traverseDropEntry(child, currentSegments, baseSubpath, files, emptyDirs);
    }
  }
}

/**
 * Recursively extracts files and empty folders from a drag-and-drop DataTransfer payload.
 */
export async function collectDroppedEntries(
  dataTransfer: DataTransfer,
  baseSubpath: string
): Promise<{ files: QueuedUpload[]; emptyDirs: string[] }> {
  const items = dataTransfer.items ? Array.from(dataTransfer.items) : [];
  const rootEntries: DropFileSystemEntry[] = [];
  for (const item of items) {
    if (item.kind !== 'file') continue;
    const entryFn = (
      item as DataTransferItem & {
        webkitGetAsEntry?: () => DropFileSystemEntry | null;
      }
    ).webkitGetAsEntry;
    if (typeof entryFn === 'function') {
      const entry = entryFn.call(item);
      if (entry) {
        rootEntries.push(entry);
      }
    }
  }

  if (rootEntries.length === 0) {
    return {
      files: buildUploadQueueFromFileList(dataTransfer.files, baseSubpath),
      emptyDirs: [],
    };
  }

  const files: QueuedUpload[] = [];
  const emptyDirs: string[] = [];
  for (const entry of rootEntries) {
    await traverseDropEntry(entry, [], baseSubpath, files, emptyDirs);
  }
  return { files, emptyDirs };
}

/**
 * Splits queued uploads into multi-file batches for small files and direct single streams for large/solo files.
 */
export function partitionUploadBatches(
  queue: QueuedUpload[],
  opts: { smallFileMaxBytes?: number; maxFiles?: number; maxBytes?: number } = {}
): UploadWorkItem[] {
  if (queue.length <= 1) {
    return queue.map((item) => ({ kind: 'single', item }));
  }

  const smallFileMaxBytes = opts.smallFileMaxBytes ?? BATCH_SMALL_FILE_MAX_BYTES;
  const maxFiles = Math.max(1, opts.maxFiles ?? BATCH_MAX_FILES);
  const maxBytes = Math.max(1, opts.maxBytes ?? BATCH_MAX_BYTES);

  const workItems: UploadWorkItem[] = [];
  const largeItems: QueuedUpload[] = [];
  let currentBatch: QueuedUpload[] = [];
  let currentBytes = 0;

  const flushBatch = () => {
    if (currentBatch.length === 0) return;
    if (currentBatch.length === 1) {
      workItems.push({ kind: 'single', item: currentBatch[0] });
    } else {
      workItems.push({ kind: 'batch', items: currentBatch });
    }
    currentBatch = [];
    currentBytes = 0;
  };

  for (const item of queue) {
    if (item.file.size > smallFileMaxBytes) {
      largeItems.push(item);
      continue;
    }
    if (
      currentBatch.length > 0 &&
      (currentBatch.length >= maxFiles || currentBytes + item.file.size > maxBytes)
    ) {
      flushBatch();
    }
    currentBatch.push(item);
    currentBytes += item.file.size;
  }
  flushBatch();

  for (const item of largeItems) {
    workItems.push({ kind: 'single', item });
  }
  return workItems;
}
