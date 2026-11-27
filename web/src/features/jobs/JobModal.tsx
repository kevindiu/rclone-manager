import {
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Copy,
  FlaskConical,
  Loader2,
  Terminal,
  X,
  XCircle,
} from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type { DraftDryRunResult, GlobalSettings, Job, RemoteInfo } from '../../types/api';
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
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<DraftDryRunResult | null>(null);
  const [error, setError] = useState('');
  const [pickerTarget, setPickerTarget] = useState<'source' | 'destination' | 'backup' | null>(
    null
  );

  const [name, setName] = useState(job?.name ?? '');
  const [description, setDescription] = useState(job?.description ?? '');
  const [enabled, setEnabled] = useState(job?.enabled ?? true);
  const [operation, setOperation] = useState<Job['operation']>(job?.operation ?? 'sync');
  const [bisyncConflictResolve, setBisyncConflictResolve] = useState<
    NonNullable<Job['bisync_conflict_resolve']>
  >(job?.bisync_conflict_resolve ?? '');
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
  const [minAge, setMinAge] = useState(job?.min_age ?? '');
  const [maxAge, setMaxAge] = useState(job?.max_age ?? '');
  const [minSize, setMinSize] = useState(job?.min_size ?? '');
  const [maxSize, setMaxSize] = useState(job?.max_size ?? '');
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
  const [fastList, setFastList] = useState(job?.fast_list ?? false);
  const [checksum, setChecksum] = useState(job?.checksum ?? false);
  const [updateOnly, setUpdateOnly] = useState(job?.update_only ?? false);
  const [serverSideAcrossConfigs, setServerSideAcrossConfigs] = useState(
    job?.server_side_across_configs ?? false
  );
  const [deleteTiming, setDeleteTiming] = useState<NonNullable<Job['delete_timing']>>(
    job?.delete_timing ?? ''
  );
  const [deleteEmptySrcDirs, setDeleteEmptySrcDirs] = useState(job?.delete_empty_src_dirs ?? false);
  const [symlinkMode, setSymlinkMode] = useState<NonNullable<Job['symlink_mode']>>(
    job?.symlink_mode ?? ''
  );
  const [sizeOnly, setSizeOnly] = useState(job?.size_only ?? false);
  const [ignoreExisting, setIgnoreExisting] = useState(job?.ignore_existing ?? false);
  const [notifyOverride, setNotifyOverride] = useState<NonNullable<Job['notify_override']>>(
    job?.notify_override ?? ''
  );

  const [cronPreview, setCronPreview] = useState<string[]>([]);
  const [cronError, setCronError] = useState('');
  const [showCmdPreview, setShowCmdPreview] = useState(false);
  const [previewDryRun, setPreviewDryRun] = useState(false);
  const [previewResync, setPreviewResync] = useState(false);
  const [previewCmd, setPreviewCmd] = useState('');
  const [previewErr, setPreviewErr] = useState('');
  const [copiedCmd, setCopiedCmd] = useState(false);

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

  const splitLines = useCallback(
    (txt: string) =>
      txt
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean),
    []
  );

  const parseOptInt = useCallback((v: string): number | null => {
    const trimmed = v.trim();
    if (!trimmed) return null;
    const n = Number(trimmed);
    return Number.isFinite(n) ? Math.trunc(n) : null;
  }, []);

  const buildPayload = useCallback(
    (): Partial<Job> => ({
      name: name.trim() || 'Preview Job',
      description: description.trim(),
      enabled,
      operation,
      source: source.trim(),
      destination: destination.trim(),
      schedule: schedule.trim(),
      exclude_patterns: splitLines(excludesText),
      include_patterns: splitLines(includesText),
      filter_rules: splitLines(filtersText),
      min_age: minAge.trim(),
      max_age: maxAge.trim(),
      min_size: minSize.trim(),
      max_size: maxSize.trim(),
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
      fast_list: fastList,
      checksum,
      update_only: updateOnly,
      server_side_across_configs: serverSideAcrossConfigs,
      delete_timing: operation === 'sync' ? deleteTiming : '',
      delete_empty_src_dirs: operation === 'move' ? deleteEmptySrcDirs : false,
      symlink_mode: symlinkMode,
      size_only: sizeOnly,
      ignore_existing: ignoreExisting,
      bisync_conflict_resolve: operation === 'bisync' ? bisyncConflictResolve : '',
      notify_override: notifyOverride,
    }),
    [
      abortIfSourceEmpty,
      backupDirBase,
      bisyncConflictResolve,
      bwlimit,
      checkers,
      checksum,
      contimeout,
      customFlags,
      cutoffMode,
      deleteEmptySrcDirs,
      deleteTiming,
      description,
      destination,
      enabled,
      excludesText,
      fastList,
      filtersText,
      ignoreExisting,
      includesText,
      lowLevelRetries,
      maxAge,
      maxDelete,
      maxSize,
      maxTransfer,
      minAge,
      minSize,
      name,
      notifyOverride,
      operation,
      parseOptInt,
      requiredMarkerFile,
      retries,
      schedule,
      serverSideAcrossConfigs,
      sizeOnly,
      source,
      splitLines,
      symlinkMode,
      timeout,
      transfers,
      updateOnly,
      versionFormat,
      versionRetentionDays,
      versioningEnabled,
    ]
  );

  useEffect(() => {
    if (!showCmdPreview || !source.trim() || !destination.trim()) {
      if (!source.trim() || !destination.trim()) {
        setPreviewCmd('');
        setPreviewErr('');
      }
      return;
    }
    const timer = window.setTimeout(() => {
      const payload = buildPayload();
      api
        .previewJobCommand(
          { ...payload, schedule: '' },
          previewDryRun,
          operation === 'bisync' && previewResync
        )
        .then((res) => {
          setPreviewCmd(res.command);
          setPreviewErr('');
        })
        .catch((err) => {
          setPreviewCmd('');
          setPreviewErr(err instanceof Error ? err.message : 'Unable to preview command');
        });
    }, 250);
    return () => window.clearTimeout(timer);
  }, [buildPayload, destination, operation, previewDryRun, previewResync, showCmdPreview, source]);

  const derivedDefaultBackupDir = () => {
    const d = destination.trim();
    if (!d) return '<destination>/_versions';
    if (d.endsWith(':')) return `${d}_versions`;
    return `${d.replace(/\/+$/, '')}/_versions`;
  };

  const handleDryRun = async () => {
    if (!source.trim()) {
      setTab('basics');
      setError('Source Path is required before running a dry run.');
      return;
    }
    if (!destination.trim()) {
      setTab('basics');
      setError('Destination Path is required before running a dry run.');
      return;
    }
    setDryRunning(true);
    setError('');
    setDryRunResult(null);
    try {
      const res = await api.dryRunDraftJob(buildPayload());
      setDryRunResult(res);
    } catch (err) {
      setDryRunResult({
        ok: false,
        duration_ms: 0,
        command: previewCmd,
        summary: 'Dry run failed',
        error: err instanceof Error ? err.message : 'Dry run failed',
      });
    } finally {
      setDryRunning(false);
    }
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setTab('basics');
      setError('Job Name is required.');
      return;
    }
    if (!source.trim()) {
      setTab('basics');
      setError('Source Path is required.');
      return;
    }
    if (!destination.trim()) {
      setTab('basics');
      setError('Destination Path is required.');
      return;
    }
    if (cronError) {
      setTab('schedule');
      setError(cronError);
      return;
    }

    setSaving(true);
    setError('');
    try {
      const payload: Partial<Job> = {
        ...buildPayload(),
        name: name.trim(),
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

  const hasGuards =
    abortIfSourceEmpty ||
    requiredMarkerFile.trim() !== '' ||
    maxDelete.trim() !== '' ||
    maxTransfer.trim() !== '' ||
    minAge.trim() !== '' ||
    maxAge.trim() !== '' ||
    minSize.trim() !== '' ||
    maxSize.trim() !== '' ||
    excludesText.trim() !== '' ||
    includesText.trim() !== '' ||
    filtersText.trim() !== '';

  const hasOverrides =
    transfers.trim() !== '' ||
    checkers.trim() !== '' ||
    retries.trim() !== '' ||
    lowLevelRetries.trim() !== '' ||
    timeout.trim() !== '' ||
    contimeout.trim() !== '' ||
    bwlimit.trim() !== '' ||
    customFlags.trim() !== '' ||
    fastList ||
    checksum ||
    updateOnly ||
    serverSideAcrossConfigs ||
    deleteTiming !== '' ||
    deleteEmptySrcDirs ||
    symlinkMode !== '' ||
    sizeOnly ||
    ignoreExisting ||
    notifyOverride !== '';

  const tabs: { key: TabKey; label: string; activeDot?: boolean }[] = [
    { key: 'basics', label: 'Basics' },
    { key: 'schedule', label: 'Schedule', activeDot: schedule.trim() !== '' },
    { key: 'versioning', label: 'Versioning', activeDot: versioningEnabled },
    { key: 'guards', label: 'Filters & Guards', activeDot: hasGuards },
    { key: 'overrides', label: 'Overrides', activeDot: hasOverrides },
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
              type="button"
              disabled={saving || dryRunning || !source.trim() || !destination.trim()}
              onClick={() => void handleDryRun()}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-sm font-medium bg-slate-800 hover:bg-slate-700 text-violet-300 disabled:opacity-50"
            >
              {dryRunning ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <FlaskConical className="w-4 h-4" />
              )}
              Dry Run
            </button>
            <button
              type="submit"
              form="job-editor-form"
              disabled={saving || dryRunning}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
            >
              {saving && <Loader2 className="w-4 h-4 animate-spin" />}
              {isEdit ? 'Save Changes' : 'Create Job'}
            </button>
          </>
        }
      >
        <form id="job-editor-form" noValidate onSubmit={handleSave} className="space-y-4">
          {error && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
              {error}
            </div>
          )}

          {dryRunResult && (
            <div
              className={`p-3 rounded-xl border text-xs space-y-2 ${
                dryRunResult.ok
                  ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-200'
                  : 'bg-rose-500/10 border-rose-500/30 text-rose-200'
              }`}
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex items-start gap-2 min-w-0">
                  {dryRunResult.ok ? (
                    <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
                  ) : (
                    <XCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
                  )}
                  <div className="min-w-0">
                    <div className="font-semibold">
                      {dryRunResult.summary} ({dryRunResult.duration_ms}ms)
                    </div>
                    {dryRunResult.error && (
                      <div className="mt-0.5 text-rose-300 break-words">{dryRunResult.error}</div>
                    )}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => setDryRunResult(null)}
                  className="text-slate-400 hover:text-slate-200 shrink-0"
                  aria-label="Dismiss dry run result"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>
              {dryRunResult.lines && dryRunResult.lines.length > 0 && (
                <pre className="max-h-28 overflow-auto p-2 rounded-lg bg-slate-950/90 border border-slate-800/80 text-[11px] font-mono text-slate-300 whitespace-pre-wrap break-words">
                  {dryRunResult.lines.join('\n')}
                </pre>
              )}
            </div>
          )}

          <div className="flex gap-1 overflow-x-auto pb-1 border-b border-slate-800">
            {tabs.map((t) => (
              <button
                key={t.key}
                type="button"
                onClick={() => setTab(t.key)}
                className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap transition-colors ${
                  tab === t.key
                    ? 'bg-sky-500/15 text-sky-300 border border-sky-500/30'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
                }`}
              >
                {t.label}
                {t.activeDot && <span className="w-1.5 h-1.5 rounded-full bg-sky-400 shrink-0" />}
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
              bisyncConflictResolve={bisyncConflictResolve}
              setBisyncConflictResolve={setBisyncConflictResolve}
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
              minAge={minAge}
              setMinAge={setMinAge}
              maxAge={maxAge}
              setMaxAge={setMaxAge}
              minSize={minSize}
              setMinSize={setMinSize}
              maxSize={maxSize}
              setMaxSize={setMaxSize}
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
              operation={operation}
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
              fastList={fastList}
              setFastList={setFastList}
              checksum={checksum}
              setChecksum={setChecksum}
              updateOnly={updateOnly}
              setUpdateOnly={setUpdateOnly}
              serverSideAcrossConfigs={serverSideAcrossConfigs}
              setServerSideAcrossConfigs={setServerSideAcrossConfigs}
              deleteTiming={deleteTiming}
              setDeleteTiming={setDeleteTiming}
              deleteEmptySrcDirs={deleteEmptySrcDirs}
              setDeleteEmptySrcDirs={setDeleteEmptySrcDirs}
              symlinkMode={symlinkMode}
              setSymlinkMode={setSymlinkMode}
              sizeOnly={sizeOnly}
              setSizeOnly={setSizeOnly}
              ignoreExisting={ignoreExisting}
              setIgnoreExisting={setIgnoreExisting}
              notifyOverride={notifyOverride}
              setNotifyOverride={setNotifyOverride}
            />
          )}

          {source.trim() && destination.trim() && (
            <div className="pt-1">
              <button
                type="button"
                onClick={() => setShowCmdPreview((v) => !v)}
                className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-slate-200"
              >
                {showCmdPreview ? (
                  <ChevronDown className="w-3.5 h-3.5" />
                ) : (
                  <ChevronRight className="w-3.5 h-3.5" />
                )}
                <Terminal className="w-3.5 h-3.5 text-sky-400" />
                Preview rclone command
              </button>

              {showCmdPreview && (
                <div className="mt-2 p-3 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
                  <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                    <div className="flex items-center gap-3">
                      <label className="inline-flex items-center gap-1.5 text-slate-400 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={previewDryRun}
                          onChange={(e) => setPreviewDryRun(e.target.checked)}
                          className="rounded accent-violet-500"
                        />
                        Dry Run
                      </label>
                      {operation === 'bisync' && (
                        <label className="inline-flex items-center gap-1.5 text-slate-400 cursor-pointer">
                          <input
                            type="checkbox"
                            checked={previewResync}
                            onChange={(e) => setPreviewResync(e.target.checked)}
                            className="rounded accent-violet-500"
                          />
                          Resync
                        </label>
                      )}
                    </div>
                    {previewCmd && (
                      <button
                        type="button"
                        onClick={() => {
                          void navigator.clipboard.writeText(previewCmd);
                          setCopiedCmd(true);
                          window.setTimeout(() => setCopiedCmd(false), 1500);
                        }}
                        className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-200"
                      >
                        {copiedCmd ? (
                          <Check className="w-3 h-3 text-emerald-400" />
                        ) : (
                          <Copy className="w-3 h-3" />
                        )}
                        {copiedCmd ? 'Copied' : 'Copy'}
                      </button>
                    )}
                  </div>
                  {previewErr ? (
                    <p className="text-xs text-rose-300 font-mono">{previewErr}</p>
                  ) : (
                    <pre className="text-[11px] font-mono text-emerald-300 overflow-x-auto whitespace-pre-wrap break-all leading-relaxed">
                      {previewCmd || 'Building command preview...'}
                    </pre>
                  )}
                </div>
              )}
            </div>
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
