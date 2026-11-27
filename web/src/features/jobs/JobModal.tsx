import { Loader2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type { GlobalSettings, Job, RemoteInfo } from '../../types/api';
import { JobBasicsTab } from './JobBasicsTab';
import { JobGuardsTab } from './JobGuardsTab';
import { JobOverridesTab } from './JobOverridesTab';
import { JobScheduleTab } from './JobScheduleTab';
import { JobVersioningTab } from './JobVersioningTab';
import { PathPickerModal } from './PathPickerModal';

interface JobModalProps {
  job?: Job | null;
  settings: GlobalSettings;
  remotes: RemoteInfo[];
  onSaved: () => void;
  onClose: () => void;
}

type TabKey = 'basics' | 'schedule' | 'versioning' | 'guards' | 'overrides';

export function JobModal({ job, settings, remotes, onSaved, onClose }: JobModalProps) {
  const isEdit = Boolean(job?.id);
  const [tab, setTab] = useState<TabKey>('basics');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [pickerTarget, setPickerTarget] = useState<'source' | 'destination' | 'backup' | null>(
    null
  );

  const [name, setName] = useState(job?.name ?? '');
  const [description, setDescription] = useState(job?.description ?? '');
  const [enabled, setEnabled] = useState(job?.enabled ?? true);
  const [operation, setOperation] = useState<'copy' | 'sync'>(job?.operation ?? 'sync');
  const [source, setSource] = useState(job?.source ?? '');
  const [destination, setDestination] = useState(job?.destination ?? '');
  const [schedule, setSchedule] = useState(job?.schedule ?? '');

  const [versioningEnabled, setVersioningEnabled] = useState(job?.versioning_enabled ?? false);
  const [backupDirBase, setBackupDirBase] = useState(job?.backup_dir_base ?? '');
  const [versionFormat, setVersionFormat] = useState(job?.version_format ?? '2006-01-02');
  const [versionRetentionDays, setVersionRetentionDays] = useState(
    job?.version_retention_days ?? 30
  );

  const [excludesText, setExcludesText] = useState((job?.exclude_patterns ?? []).join('\n'));
  const [includesText, setIncludesText] = useState((job?.include_patterns ?? []).join('\n'));
  const [filtersText, setFiltersText] = useState((job?.filter_rules ?? []).join('\n'));
  const [maxTransfer, setMaxTransfer] = useState(job?.max_transfer ?? '');
  const [cutoffMode, setCutoffMode] = useState<'soft' | 'hard' | 'cautious' | ''>(
    job?.cutoff_mode ?? 'soft'
  );
  const [abortIfSourceEmpty, setAbortIfSourceEmpty] = useState(job?.abort_if_source_empty ?? false);
  const [requiredMarkerFile, setRequiredMarkerFile] = useState(job?.required_marker_file ?? '');
  const [maxDelete, setMaxDelete] = useState(
    job?.max_delete !== undefined && job?.max_delete !== null ? String(job.max_delete) : ''
  );

  const [transfers, setTransfers] = useState(
    job?.transfers !== undefined && job?.transfers !== null ? String(job.transfers) : ''
  );
  const [checkers, setCheckers] = useState(
    job?.checkers !== undefined && job?.checkers !== null ? String(job.checkers) : ''
  );
  const [bwlimit, setBwlimit] = useState(job?.bwlimit ?? '');
  const [retries, setRetries] = useState(
    job?.retries !== undefined && job?.retries !== null ? String(job.retries) : ''
  );
  const [lowLevelRetries, setLowLevelRetries] = useState(
    job?.low_level_retries !== undefined && job?.low_level_retries !== null
      ? String(job.low_level_retries)
      : ''
  );
  const [timeout, setTimeoutVal] = useState(job?.timeout ?? '');
  const [contimeout, setContimeout] = useState(job?.contimeout ?? '');
  const [customFlags, setCustomFlags] = useState(job?.custom_flags ?? '');

  const [cronPreview, setCronPreview] = useState<string[]>([]);
  const [cronError, setCronError] = useState('');

  useEffect(() => {
    const trimmed = schedule.trim();
    if (!trimmed) {
      setCronPreview([]);
      setCronError('');
      return;
    }
    const timer = window.setTimeout(() => {
      api
        .previewCron(trimmed, settings.timezone, 5)
        .then((res) => {
          setCronPreview(res.next ?? []);
          setCronError('');
        })
        .catch((err) => {
          setCronPreview([]);
          setCronError(err instanceof Error ? err.message : 'Invalid cron expression');
        });
    }, 250);
    return () => window.clearTimeout(timer);
  }, [schedule, settings.timezone]);

  const splitLines = (txt: string) =>
    txt
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean);

  const parseOptInt = (v: string): number | null => {
    const trimmed = v.trim();
    if (!trimmed) return null;
    const n = Number(trimmed);
    return Number.isFinite(n) ? Math.trunc(n) : null;
  };

  const derivedDefaultBackupDir = () => {
    const d = destination.trim();
    if (!d) return '<destination>/_versions';
    if (d.endsWith(':')) return `${d}_versions`;
    return `${d.replace(/\/+$/, '')}/_versions`;
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError('');
    try {
      const payload: Partial<Job> = {
        name: name.trim(),
        description: description.trim(),
        enabled,
        operation,
        source: source.trim(),
        destination: destination.trim(),
        schedule: schedule.trim(),
        exclude_patterns: splitLines(excludesText),
        include_patterns: splitLines(includesText),
        filter_rules: splitLines(filtersText),
        custom_flags: customFlags.trim(),
        max_transfer: maxTransfer.trim(),
        cutoff_mode: maxTransfer.trim() ? cutoffMode || 'soft' : '',
        versioning_enabled: versioningEnabled,
        backup_dir_base: backupDirBase.trim(),
        version_format: versionFormat.trim() || '2006-01-02',
        version_retention_days: Math.max(0, Number(versionRetentionDays) || 0),
        abort_if_source_empty: abortIfSourceEmpty,
        required_marker_file: requiredMarkerFile.trim(),
        max_delete: parseOptInt(maxDelete),
        transfers: parseOptInt(transfers),
        checkers: parseOptInt(checkers),
        bwlimit: bwlimit.trim(),
        retries: parseOptInt(retries),
        low_level_retries: parseOptInt(lowLevelRetries),
        timeout: timeout.trim(),
        contimeout: contimeout.trim(),
      };

      if (isEdit && job?.id) {
        await api.updateJob(job.id, payload);
      } else {
        await api.createJob(payload);
      }
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save job');
    } finally {
      setSaving(false);
    }
  };

  const tabs: { key: TabKey; label: string }[] = [
    { key: 'basics', label: 'Basics' },
    { key: 'schedule', label: 'Schedule' },
    { key: 'versioning', label: 'Versioning' },
    { key: 'guards', label: 'Filters & Guards' },
    { key: 'overrides', label: 'Overrides' },
  ];

  return (
    <>
      <Modal
        title={isEdit ? `Edit Job: ${job?.name}` : 'Create Backup Job'}
        subtitle="Configure source, destination, schedule, versioning, and safety guards"
        onClose={onClose}
        maxWidth="max-w-3xl"
        footer={
          <>
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded-lg text-sm font-medium text-slate-300 hover:bg-slate-800"
            >
              Cancel
            </button>
            <button
              type="submit"
              form="job-editor-form"
              disabled={saving}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
            >
              {saving && <Loader2 className="w-4 h-4 animate-spin" />}
              {isEdit ? 'Save Changes' : 'Create Job'}
            </button>
          </>
        }
      >
        <form id="job-editor-form" onSubmit={handleSave} className="space-y-4">
          {error && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
              {error}
            </div>
          )}

          <div className="flex gap-1 overflow-x-auto pb-1 border-b border-slate-800">
            {tabs.map((t) => (
              <button
                key={t.key}
                type="button"
                onClick={() => setTab(t.key)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap transition-colors ${
                  tab === t.key
                    ? 'bg-sky-500/15 text-sky-300 border border-sky-500/30'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          {tab === 'basics' && (
            <JobBasicsTab
              name={name}
              setName={setName}
              description={description}
              setDescription={setDescription}
              operation={operation}
              setOperation={setOperation}
              source={source}
              setSource={setSource}
              destination={destination}
              setDestination={setDestination}
              enabled={enabled}
              setEnabled={setEnabled}
              onPickPath={setPickerTarget}
            />
          )}

          {tab === 'schedule' && (
            <JobScheduleTab
              schedule={schedule}
              setSchedule={setSchedule}
              timezone={settings.timezone}
              cronError={cronError}
              cronPreview={cronPreview}
            />
          )}

          {tab === 'versioning' && (
            <JobVersioningTab
              versioningEnabled={versioningEnabled}
              setVersioningEnabled={setVersioningEnabled}
              backupDirBase={backupDirBase}
              setBackupDirBase={setBackupDirBase}
              versionFormat={versionFormat}
              setVersionFormat={setVersionFormat}
              versionRetentionDays={versionRetentionDays}
              setVersionRetentionDays={setVersionRetentionDays}
              derivedDefaultBackupDir={derivedDefaultBackupDir()}
              onPickBackupDir={() => setPickerTarget('backup')}
            />
          )}

          {tab === 'guards' && (
            <JobGuardsTab
              abortIfSourceEmpty={abortIfSourceEmpty}
              setAbortIfSourceEmpty={setAbortIfSourceEmpty}
              requiredMarkerFile={requiredMarkerFile}
              setRequiredMarkerFile={setRequiredMarkerFile}
              maxDelete={maxDelete}
              setMaxDelete={setMaxDelete}
              maxTransfer={maxTransfer}
              setMaxTransfer={setMaxTransfer}
              cutoffMode={cutoffMode}
              setCutoffMode={setCutoffMode}
              excludesText={excludesText}
              setExcludesText={setExcludesText}
              includesText={includesText}
              setIncludesText={setIncludesText}
              filtersText={filtersText}
              setFiltersText={setFiltersText}
            />
          )}

          {tab === 'overrides' && (
            <JobOverridesTab
              settings={settings}
              transfers={transfers}
              setTransfers={setTransfers}
              checkers={checkers}
              setCheckers={setCheckers}
              retries={retries}
              setRetries={setRetries}
              lowLevelRetries={lowLevelRetries}
              setLowLevelRetries={setLowLevelRetries}
              timeout={timeout}
              setTimeoutVal={setTimeoutVal}
              contimeout={contimeout}
              setContimeout={setContimeout}
              bwlimit={bwlimit}
              setBwlimit={setBwlimit}
              customFlags={customFlags}
              setCustomFlags={setCustomFlags}
            />
          )}
        </form>
      </Modal>

      {pickerTarget && (
        <PathPickerModal
          remotes={remotes}
          initialValue={
            pickerTarget === 'source'
              ? source
              : pickerTarget === 'destination'
                ? destination
                : backupDirBase
          }
          onSelect={(val) => {
            if (pickerTarget === 'source') setSource(val);
            else if (pickerTarget === 'destination') setDestination(val);
            else setBackupDirBase(val);
          }}
          onClose={() => setPickerTarget(null)}
        />
      )}
    </>
  );
}
