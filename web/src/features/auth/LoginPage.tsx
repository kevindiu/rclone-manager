import { AlertCircle, Eye, EyeOff, Loader2, Lock, ShieldCheck, UserPlus } from 'lucide-react';
import { useState } from 'react';
import { api } from '../../api/client';
import type { AuthUser } from '../../types/api';

export function LoginPage({
  setupRequired,
  onAuthenticated,
}: {
  setupRequired: boolean;
  onAuthenticated: (user: AuthUser) => void;
}) {
  const [username, setUsername] = useState(setupRequired ? 'admin' : '');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const trimmedUser = username.trim();
    if (!trimmedUser) {
      setError('Username is required.');
      return;
    }

    if (setupRequired) {
      if (password.length < 8) {
        setError('Password must be at least 8 characters.');
        return;
      }
      if (password !== confirmPassword) {
        setError('Passwords do not match.');
        return;
      }
    } else if (!password) {
      setError('Password is required.');
      return;
    }

    setSubmitting(true);
    try {
      const res = setupRequired
        ? await api.authSetup(trimmedUser, password)
        : await api.authLogin(trimmedUser, password);
      onAuthenticated(res.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Authentication failed');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-dvh bg-slate-950 text-slate-100 flex items-center justify-center p-4">
      <div className="w-full max-w-md rounded-2xl bg-slate-900/90 border border-slate-800 shadow-2xl p-6 sm:p-7 space-y-6">
        <div className="flex items-center gap-3">
          <img src="/icon.svg" alt="rclone-manager logo" className="w-10 h-10 rounded-xl" />
          <div>
            <h1 className="text-lg font-bold tracking-tight text-slate-100">rclone-manager</h1>
            <p className="text-xs text-slate-400">
              {setupRequired ? 'Initial Administrator Setup' : 'Authentication Required'}
            </p>
          </div>
        </div>

        {setupRequired && (
          <div className="p-3.5 rounded-xl bg-sky-500/10 border border-sky-500/30 text-xs text-sky-200 flex items-start gap-2.5">
            <ShieldCheck className="w-4 h-4 text-sky-400 shrink-0 mt-0.5" />
            <span>
              Create your local administrator account to protect remotes, backup jobs, and file
              restores. Authentication is always required.
            </span>
          </div>
        )}

        {error && (
          <div
            role="alert"
            className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-200 flex items-start gap-2"
          >
            <AlertCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label
              htmlFor="auth-username"
              className="block text-xs font-medium text-slate-300 mb-1"
            >
              Username
            </label>
            <input
              id="auth-username"
              type="text"
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="admin"
              disabled={submitting}
              className="w-full px-3.5 py-2.5 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100 placeholder:text-slate-600 focus:outline-none focus:border-sky-500"
            />
          </div>

          <div>
            <label
              htmlFor="auth-password"
              className="block text-xs font-medium text-slate-300 mb-1"
            >
              Password
            </label>
            <div className="relative">
              <input
                id="auth-password"
                type={showPassword ? 'text' : 'password'}
                autoComplete={setupRequired ? 'new-password' : 'current-password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={setupRequired ? 'Minimum 8 characters' : 'Enter your password'}
                disabled={submitting}
                className="w-full pl-3.5 pr-10 py-2.5 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100 placeholder:text-slate-600 focus:outline-none focus:border-sky-500"
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? 'Hide password' : 'Show password'}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 p-1 text-slate-400 hover:text-slate-200"
              >
                {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          {setupRequired && (
            <div>
              <label
                htmlFor="auth-confirm-password"
                className="block text-xs font-medium text-slate-300 mb-1"
              >
                Confirm Password
              </label>
              <input
                id="auth-confirm-password"
                type={showPassword ? 'text' : 'password'}
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Re-enter password"
                disabled={submitting}
                className="w-full px-3.5 py-2.5 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100 placeholder:text-slate-600 focus:outline-none focus:border-sky-500"
              />
            </div>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="w-full inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50 transition-colors"
          >
            {submitting ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : setupRequired ? (
              <UserPlus className="w-4 h-4" />
            ) : (
              <Lock className="w-4 h-4" />
            )}
            {setupRequired ? 'Create Account & Sign In' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  );
}
