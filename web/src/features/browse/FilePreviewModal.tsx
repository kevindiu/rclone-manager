import { Download, Loader2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type { BrowseEntry } from '../../types/api';
import { formatBytes } from '../../utils/format';

const IMAGE_EXTS = new Set(['.png', '.jpg', '.jpeg', '.gif', '.webp']);
const TEXT_EXTS = new Set([
  '.txt',
  '.md',
  '.log',
  '.json',
  '.yaml',
  '.yml',
  '.toml',
  '.ini',
  '.conf',
  '.cfg',
  '.csv',
  '.xml',
  '.sh',
  '.bash',
  '.zsh',
  '.go',
  '.ts',
  '.tsx',
  '.js',
  '.jsx',
  '.py',
  '.rs',
  '.c',
  '.h',
  '.cpp',
  '.java',
  '.css',
  '.sql',
  '.env',
  '.gitignore',
]);
const TEXT_BASENAMES = new Set(['dockerfile', 'makefile', '.gitignore', '.env']);
const MAX_TEXT_PREVIEW_BYTES = 512 * 1024;
const MAX_PDF_PREVIEW_BYTES = 25 * 1024 * 1024;

function getPreviewKind(filename: string): 'image' | 'pdf' | 'text' | 'unsupported' {
  const lower = filename.toLowerCase();
  const base = lower.split('/').pop() ?? lower;
  if (TEXT_BASENAMES.has(base)) return 'text';
  const dot = base.lastIndexOf('.');
  const ext = dot >= 0 ? base.slice(dot) : '';
  if (IMAGE_EXTS.has(ext)) return 'image';
  if (ext === '.pdf') return 'pdf';
  if (TEXT_EXTS.has(ext)) return 'text';
  return 'unsupported';
}

export function isPreviewableFile(entry: BrowseEntry): boolean {
  if (entry.IsDir) return false;
  return getPreviewKind(entry.Name) !== 'unsupported';
}

export function FilePreviewModal({
  remote,
  entry,
  fullPath,
  onClose,
}: {
  remote: string;
  entry: BrowseEntry;
  fullPath: string;
  onClose: () => void;
}) {
  const kind = getPreviewKind(entry.Name);
  const inlineUrl = api.previewUrl(remote, fullPath);
  const downloadHref = api.downloadUrl(remote, fullPath);
  const tooLargeForText = kind === 'text' && entry.Size > MAX_TEXT_PREVIEW_BYTES;
  const tooLargeForPdf = kind === 'pdf' && entry.Size > MAX_PDF_PREVIEW_BYTES;

  const [textContent, setTextContent] = useState('');
  const [pdfUrl, setPdfUrl] = useState('');
  const [loading, setLoading] = useState(
    (kind === 'text' && !tooLargeForText) || (kind === 'pdf' && !tooLargeForPdf)
  );
  const [error, setError] = useState('');

  useEffect(() => {
    if (kind === 'text' && !tooLargeForText) {
      let cancelled = false;
      setLoading(true);
      setError('');
      setTextContent('');
      fetch(inlineUrl, { credentials: 'same-origin' })
        .then(async (res) => {
          if (!res.ok) {
            throw new Error(`HTTP ${res.status}`);
          }
          return res.text();
        })
        .then((txt) => {
          if (!cancelled) setTextContent(txt);
        })
        .catch((err) => {
          if (!cancelled)
            setError(err instanceof Error ? err.message : 'Failed to load file preview');
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
      return () => {
        cancelled = true;
      };
    }

    if (kind === 'pdf' && !tooLargeForPdf) {
      let cancelled = false;
      let objectUrl = '';
      setLoading(true);
      setError('');
      setPdfUrl('');
      fetch(inlineUrl, { credentials: 'same-origin' })
        .then(async (res) => {
          if (!res.ok) {
            throw new Error(`HTTP ${res.status}`);
          }
          const buf = await res.arrayBuffer();
          return new Blob([buf], { type: 'application/pdf' });
        })
        .then((blob) => {
          if (cancelled) return;
          objectUrl = URL.createObjectURL(blob);
          setPdfUrl(objectUrl);
        })
        .catch((err) => {
          if (!cancelled)
            setError(err instanceof Error ? err.message : 'Failed to load PDF preview');
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
      return () => {
        cancelled = true;
        if (objectUrl) URL.revokeObjectURL(objectUrl);
      };
    }

    setLoading(false);
    return undefined;
  }, [inlineUrl, kind, tooLargeForPdf, tooLargeForText]);

  return (
    <Modal
      title={entry.Name}
      subtitle={`${remote}:${fullPath} · ${formatBytes(entry.Size)}`}
      onClose={onClose}
      maxWidth="max-w-4xl"
      footer={
        <>
          <a
            href={downloadHref}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-sm font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
          >
            <Download className="w-4 h-4 text-sky-400" />
            Download
          </a>
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-sm font-medium bg-sky-500 hover:bg-sky-400 text-slate-950"
          >
            Close
          </button>
        </>
      }
    >
      {kind === 'image' && (
        <div className="flex items-center justify-center p-2 rounded-xl bg-slate-950 border border-slate-800">
          <img
            src={inlineUrl}
            alt={entry.Name}
            className="max-h-[65vh] max-w-full rounded-lg object-contain"
          />
        </div>
      )}

      {kind === 'pdf' && (
        <div className="rounded-xl bg-slate-950 border border-slate-800 overflow-hidden">
          {tooLargeForPdf ? (
            <p className="text-xs text-slate-400 py-6 px-4 text-center">
              PDF is {formatBytes(entry.Size)}, which exceeds the 25 MiB inline preview limit. Use
              Download instead.
            </p>
          ) : loading ? (
            <div className="flex items-center justify-center py-12 text-xs text-slate-400 gap-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              Loading PDF preview...
            </div>
          ) : error ? (
            <p className="text-xs text-rose-300 py-6 px-4 text-center">{error}</p>
          ) : pdfUrl ? (
            <iframe src={pdfUrl} title={entry.Name} className="w-full h-[65vh] bg-white border-0" />
          ) : null}
        </div>
      )}

      {kind === 'text' && (
        <div className="rounded-xl bg-slate-950 border border-slate-800 p-3.5">
          {tooLargeForText ? (
            <p className="text-xs text-slate-400 py-6 text-center">
              File is {formatBytes(entry.Size)}, which exceeds the 512 KiB inline text preview
              limit. Use Download instead.
            </p>
          ) : loading ? (
            <div className="flex items-center justify-center py-12 text-xs text-slate-400 gap-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              Loading preview...
            </div>
          ) : error ? (
            <p className="text-xs text-rose-300 py-6 text-center">{error}</p>
          ) : (
            <pre className="max-h-[60vh] overflow-auto text-xs font-mono text-slate-200 whitespace-pre-wrap break-words leading-relaxed">
              {textContent}
            </pre>
          )}
        </div>
      )}

      {kind === 'unsupported' && (
        <div className="rounded-xl bg-slate-950 border border-slate-800 p-8 text-center text-xs text-slate-400">
          Inline preview is not enabled for this file type. Use Download to inspect it locally.
        </div>
      )}
    </Modal>
  );
}
