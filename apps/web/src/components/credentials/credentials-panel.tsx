'use client';

import React, { useCallback, useEffect, useState } from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  Eye,
  EyeOff,
  Key,
  Loader2,
  Lock,
  Plus,
  RefreshCw,
  Trash2,
  X,
} from 'lucide-react';
import { ApiError, api } from '../../lib/api';

// ─── Types ───────────────────────────────────────────────────────────────────

export type CredentialKind = 'api_key' | 'oauth_token' | 'git_token';
export type SecretSource = 'env' | 'inline_encrypted';

export interface Credential {
  id: string;
  name: string;
  kind: CredentialKind;
  providerId?: string;
  description?: string;
  secretSource: SecretSource;
  createdAt: string;
}

interface CredentialListResponse {
  items: Credential[];
  count: number;
}

interface CredentialCreateBody {
  name: string;
  kind: string;
  secretSource: string;
  secretValue: string;
  description?: string;
}

// ─── Provider shortcuts ───────────────────────────────────────────────────────

interface ProviderShortcut {
  label: string;
  name: string;
  description: string;
  kind: CredentialKind;
  envName?: string;
}

const PROVIDER_SHORTCUTS: ProviderShortcut[] = [
  { label: 'OpenRouter', name: 'openrouter-key', description: 'API key de OpenRouter para acceso a modelos LLM', kind: 'api_key', envName: 'OPENROUTER_API_KEY' },
  { label: 'Anthropic', name: 'anthropic-key', description: 'API key de Anthropic (Claude)', kind: 'api_key', envName: 'ANTHROPIC_API_KEY' },
  { label: 'OpenAI', name: 'openai-key', description: 'API key de OpenAI (GPT)', kind: 'api_key', envName: 'OPENAI_API_KEY' },
  { label: 'Gemini', name: 'gemini-key', description: 'API key de Google Gemini', kind: 'api_key', envName: 'GEMINI_API_KEY' },
];

// ─── Helpers ─────────────────────────────────────────────────────────────────

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('es', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function kindLabel(kind: CredentialKind): string {
  switch (kind) {
    case 'api_key': return 'API Key';
    case 'oauth_token': return 'OAuth Token';
    case 'git_token': return 'Git Token';
  }
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function SecretSourceBadge({ source }: { source: SecretSource }) {
  if (source === 'inline_encrypted') {
    return (
      <span className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-semibold bg-indigo-500/10 text-indigo-300 border border-indigo-500/20">
        <Lock size={9} /> Encriptada
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-semibold bg-amber-500/10 text-amber-300 border border-amber-500/20">
      <Eye size={9} /> Env
    </span>
  );
}

function KindBadge({ kind }: { kind: CredentialKind }) {
  return (
    <span className="rounded px-1.5 py-0.5 text-[10px] font-semibold bg-gray-800 text-gray-300 border border-gray-700">
      {kindLabel(kind)}
    </span>
  );
}

// ─── Delete Confirm Dialog ────────────────────────────────────────────────────

interface DeleteDialogProps {
  name: string;
  onConfirm: () => void;
  onCancel: () => void;
  loading: boolean;
}

function DeleteDialog({ name, onConfirm, onCancel, loading }: DeleteDialogProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="w-full max-w-sm rounded-2xl border border-gray-800 bg-gray-950 p-6 shadow-2xl">
        <div className="mb-4 flex items-start gap-3">
          <AlertTriangle size={20} className="mt-0.5 shrink-0 text-rose-400" />
          <div>
            <h3 className="text-sm font-bold text-white">Eliminar credencial</h3>
            <p className="mt-1 text-xs text-muted-foreground">
              ¿Confirmas que querés eliminar{' '}
              <span className="font-mono text-white">{name}</span>? Esta acción no se puede deshacer.
            </p>
          </div>
        </div>
        <div className="flex justify-end gap-2">
          <button
            onClick={onCancel}
            disabled={loading}
            className="rounded-lg border border-gray-800 bg-transparent px-4 py-2 text-xs font-semibold text-gray-300 hover:bg-gray-900 disabled:opacity-40"
          >
            Cancelar
          </button>
          <button
            onClick={onConfirm}
            disabled={loading}
            className="inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-4 py-2 text-xs font-bold text-white hover:bg-rose-500 disabled:opacity-40"
          >
            {loading ? <Loader2 size={12} className="animate-spin" /> : <Trash2 size={12} />}
            Eliminar
          </button>
        </div>
      </div>
    </div>
  );
}

// ─── Add Credential Form ──────────────────────────────────────────────────────

interface FormState {
  name: string;
  kind: CredentialKind;
  secretSource: SecretSource;
  secretValue: string;
  envVarName: string;
  description: string;
}

const EMPTY_FORM: FormState = {
  name: '',
  kind: 'api_key',
  secretSource: 'inline_encrypted',
  secretValue: '',
  envVarName: '',
  description: '',
};

interface AddFormProps {
  onSaved: () => void;
  onCancel: () => void;
}

function AddCredentialForm({ onSaved, onCancel }: AddFormProps) {
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [showSecret, setShowSecret] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');

  function applyShortcut(s: ProviderShortcut) {
    setForm(prev => ({
      ...prev,
      name: s.name,
      kind: s.kind,
      description: s.description,
      envVarName: s.envName ?? '',
    }));
  }

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm(prev => ({ ...prev, [key]: value }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaveError('');

    const secretValue =
      form.secretSource === 'env' ? form.envVarName.trim() : form.secretValue;

    if (!form.name.trim()) { setSaveError('El nombre es obligatorio.'); return; }
    if (!secretValue) {
      setSaveError(form.secretSource === 'env' ? 'El nombre de la variable de entorno es obligatorio.' : 'El valor del secreto es obligatorio.');
      return;
    }

    const body: CredentialCreateBody = {
      name: form.name.trim(),
      kind: form.kind,
      secretSource: form.secretSource,
      secretValue,
      ...(form.description.trim() ? { description: form.description.trim() } : {}),
    };

    setSaving(true);
    try {
      await api.post('/credentials', body);
      onSaved();
    } catch (err) {
      setSaveError(errorMessage(err, 'Error al guardar la credencial.'));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="glass-panel rounded-2xl border border-primary/20 bg-gray-950/80 p-6">
      <div className="mb-5 flex items-center justify-between">
        <h3 className="flex items-center gap-2 text-sm font-bold text-white">
          <Plus size={16} className="text-primary" /> Agregar credencial
        </h3>
        <button
          onClick={onCancel}
          className="rounded-lg p-1 text-muted-foreground hover:bg-gray-900 hover:text-white"
        >
          <X size={16} />
        </button>
      </div>

      {/* Provider shortcuts */}
      <div className="mb-5">
        <p className="mb-2 text-[10px] uppercase tracking-widest text-muted-foreground font-semibold">Proveedores comunes</p>
        <div className="flex flex-wrap gap-2">
          {PROVIDER_SHORTCUTS.map(s => (
            <button
              key={s.name}
              type="button"
              onClick={() => applyShortcut(s)}
              className="rounded-full border border-gray-700 bg-gray-900 px-3 py-1 text-[11px] font-semibold text-gray-300 hover:border-primary/50 hover:text-primary transition-colors"
            >
              {s.label}
            </button>
          ))}
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Nombre */}
        <div>
          <label className="mb-1.5 block text-xs font-semibold text-gray-300">
            Nombre <span className="text-rose-400">*</span>
          </label>
          <input
            type="text"
            value={form.name}
            onChange={e => set('name', e.target.value)}
            placeholder="ej: openrouter-key"
            className="w-full rounded-lg border border-gray-800 bg-gray-950 px-3 py-2 font-mono text-xs text-white outline-none placeholder:text-gray-600 focus:border-primary/60"
          />
        </div>

        {/* Tipo */}
        <div>
          <label className="mb-1.5 block text-xs font-semibold text-gray-300">Tipo</label>
          <select
            value={form.kind}
            onChange={e => set('kind', e.target.value as CredentialKind)}
            className="w-full rounded-lg border border-gray-800 bg-gray-950 px-3 py-2 text-xs text-white outline-none focus:border-primary/60 cursor-pointer"
          >
            <option value="api_key">API Key</option>
            <option value="oauth_token">OAuth Token</option>
            <option value="git_token">Git Token</option>
          </select>
        </div>

        {/* Fuente */}
        <div>
          <label className="mb-1.5 block text-xs font-semibold text-gray-300">Fuente del secreto</label>
          <div className="flex gap-3">
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="secretSource"
                value="inline_encrypted"
                checked={form.secretSource === 'inline_encrypted'}
                onChange={() => set('secretSource', 'inline_encrypted')}
                className="accent-primary"
              />
              <span className="text-xs text-gray-300">Encriptada en SQLite</span>
            </label>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="secretSource"
                value="env"
                checked={form.secretSource === 'env'}
                onChange={() => set('secretSource', 'env')}
                className="accent-primary"
              />
              <span className="text-xs text-gray-300">Variable de entorno</span>
            </label>
          </div>
        </div>

        {/* Valor del secreto o nombre de env var */}
        {form.secretSource === 'inline_encrypted' ? (
          <div>
            <label className="mb-1.5 block text-xs font-semibold text-gray-300">
              Valor del secreto <span className="text-rose-400">*</span>
            </label>
            <div className="relative">
              <input
                type={showSecret ? 'text' : 'password'}
                value={form.secretValue}
                onChange={e => set('secretValue', e.target.value)}
                placeholder="sk-..."
                className="w-full rounded-lg border border-gray-800 bg-gray-950 px-3 py-2 pr-10 font-mono text-xs text-white outline-none placeholder:text-gray-600 focus:border-primary/60"
              />
              <button
                type="button"
                onClick={() => setShowSecret(v => !v)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-white"
              >
                {showSecret ? <EyeOff size={13} /> : <Eye size={13} />}
              </button>
            </div>
            <p className="mt-1 text-[10px] text-muted-foreground">El valor se encripta server-side antes de guardarse en la DB.</p>
          </div>
        ) : (
          <div>
            <label className="mb-1.5 block text-xs font-semibold text-gray-300">
              Nombre de la variable <span className="text-rose-400">*</span>
            </label>
            <input
              type="text"
              value={form.envVarName}
              onChange={e => set('envVarName', e.target.value)}
              placeholder="OPENROUTER_API_KEY"
              className="w-full rounded-lg border border-gray-800 bg-gray-950 px-3 py-2 font-mono text-xs text-white outline-none placeholder:text-gray-600 focus:border-primary/60"
            />
            <p className="mt-1 text-[10px] text-muted-foreground">BattOS leerá esta variable del entorno del servidor al momento de usarla.</p>
          </div>
        )}

        {/* Descripción */}
        <div>
          <label className="mb-1.5 block text-xs font-semibold text-gray-300">Descripción (opcional)</label>
          <input
            type="text"
            value={form.description}
            onChange={e => set('description', e.target.value)}
            placeholder="Para qué se usa esta credencial..."
            className="w-full rounded-lg border border-gray-800 bg-gray-950 px-3 py-2 text-xs text-white outline-none placeholder:text-gray-600 focus:border-primary/60"
          />
        </div>

        {saveError && (
          <div className="flex items-center gap-2 rounded-lg border border-rose-500/20 bg-rose-500/10 px-3 py-2 text-xs text-rose-300">
            <AlertTriangle size={12} className="shrink-0" />
            {saveError}
          </div>
        )}

        <div className="flex justify-end gap-2 pt-1">
          <button
            type="button"
            onClick={onCancel}
            disabled={saving}
            className="rounded-lg border border-gray-800 bg-transparent px-4 py-2 text-xs font-semibold text-gray-300 hover:bg-gray-900 disabled:opacity-40"
          >
            Cancelar
          </button>
          <button
            type="submit"
            disabled={saving}
            className="inline-flex items-center gap-1.5 rounded-lg bg-primary px-4 py-2 text-xs font-bold text-primary-foreground hover:bg-yellow-400 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {saving ? <Loader2 size={12} className="animate-spin" /> : <CheckCircle2 size={12} />}
            Guardar
          </button>
        </div>
      </form>
    </div>
  );
}

// ─── Main Panel ───────────────────────────────────────────────────────────────

export default function CredentialsPanel() {
  const [credentials, setCredentials] = useState<Credential[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState('');
  const [featurePending, setFeaturePending] = useState(false);

  const [showForm, setShowForm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Credential | null>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchCredentials = useCallback(async () => {
    setLoading(true);
    setErrorMsg('');
    setFeaturePending(false);
    try {
      const res = await api.get<CredentialListResponse>('/credentials');
      setCredentials(res.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setFeaturePending(true);
        setCredentials([]);
      } else {
        setErrorMsg(errorMessage(err, 'Error al cargar las credenciales.'));
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchCredentials(); }, [fetchCredentials]);

  async function handleDelete(cred: Credential) {
    setDeleting(true);
    try {
      await api.delete(`/credentials/${cred.name}`);
      setDeleteTarget(null);
      await fetchCredentials();
    } catch (err) {
      setErrorMsg(errorMessage(err, 'Error al eliminar la credencial.'));
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  }

  async function handleSaved() {
    setShowForm(false);
    await fetchCredentials();
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <section className="glass-panel rounded-2xl border border-gray-800 p-6">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="flex items-center gap-2 text-primary">
              <Key size={18} />
              <p className="text-xs font-bold uppercase tracking-widest">Credenciales</p>
            </div>
            <h2 className="mt-2 text-2xl font-bold text-white">API Keys & Secretos</h2>
            <p className="mt-2 max-w-3xl text-sm text-muted-foreground">
              Administrá las credenciales que BattOS usa para conectarse a proveedores externos. Los secretos encriptados nunca se devuelven al dashboard.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={fetchCredentials}
              disabled={loading}
              className="inline-flex items-center justify-center gap-2 rounded-lg border border-gray-800 bg-gray-950 px-4 py-2 text-xs font-bold uppercase tracking-wide text-gray-200 hover:border-primary/50 hover:text-primary disabled:opacity-40"
            >
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
              Refrescar
            </button>
            <button
              onClick={() => setShowForm(v => !v)}
              className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-xs font-bold uppercase tracking-wide text-primary-foreground hover:bg-yellow-400"
            >
              <Plus size={14} />
              Agregar
            </button>
          </div>
        </div>
      </section>

      {/* Add form */}
      {showForm && (
        <AddCredentialForm
          onSaved={handleSaved}
          onCancel={() => setShowForm(false)}
        />
      )}

      {/* Error banner */}
      {errorMsg && (
        <div className="flex items-center gap-3 rounded-xl border border-rose-500/20 bg-rose-500/10 px-4 py-3 text-xs text-rose-300">
          <AlertTriangle size={14} className="shrink-0" />
          {errorMsg}
          <button onClick={() => setErrorMsg('')} className="ml-auto text-muted-foreground hover:text-white">
            <X size={13} />
          </button>
        </div>
      )}

      {/* Feature pending */}
      {featurePending && (
        <div className="flex items-center gap-3 rounded-xl border border-amber-500/20 bg-amber-500/10 px-4 py-3 text-xs text-amber-200">
          <AlertTriangle size={14} className="shrink-0 text-amber-300" />
          El endpoint <span className="font-mono">/credentials</span> aún no está disponible. Esta sección estará activa cuando el backend lo implemente.
        </div>
      )}

      {/* Credentials list */}
      <section className="glass-panel rounded-2xl border border-gray-800 p-5">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="flex items-center gap-2 text-sm font-bold text-white">
            <Lock size={14} className="text-primary" />
            Credenciales almacenadas
          </h3>
          {!loading && !featurePending && (
            <span className="text-[10px] text-muted-foreground">{credentials.length} {credentials.length === 1 ? 'credencial' : 'credenciales'}</span>
          )}
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12 text-muted-foreground">
            <Loader2 size={20} className="animate-spin mr-2" />
            <span className="text-xs">Cargando credenciales...</span>
          </div>
        ) : featurePending ? (
          <div className="py-10 text-center">
            <Key size={32} className="mx-auto mb-3 text-gray-700" />
            <p className="text-xs text-muted-foreground">El feature de credenciales está pendiente de implementación en el backend.</p>
          </div>
        ) : credentials.length === 0 ? (
          <div className="py-10 text-center">
            <Key size={32} className="mx-auto mb-3 text-gray-700" />
            <p className="text-sm font-semibold text-gray-400">No hay credenciales.</p>
            <p className="mt-1 text-xs text-muted-foreground">Agregá tu primera API key usando el botón "Agregar".</p>
          </div>
        ) : (
          <div className="space-y-2">
            {credentials.map(cred => (
              <div
                key={cred.id}
                className="flex items-start justify-between gap-3 rounded-xl border border-gray-800/60 bg-gray-900/50 p-4"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-mono text-sm font-bold text-white">{cred.name}</span>
                    <KindBadge kind={cred.kind} />
                    <SecretSourceBadge source={cred.secretSource} />
                  </div>
                  {cred.description && (
                    <p className="mt-1 text-xs text-muted-foreground">{cred.description}</p>
                  )}
                  <p className="mt-1.5 text-[10px] text-gray-600">
                    Agregada {formatDate(cred.createdAt)}
                  </p>
                </div>
                <button
                  onClick={() => setDeleteTarget(cred)}
                  title="Eliminar credencial"
                  className="shrink-0 rounded-lg p-2 text-gray-600 hover:bg-rose-500/10 hover:text-rose-400 transition-colors"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Delete confirm dialog */}
      {deleteTarget && (
        <DeleteDialog
          name={deleteTarget.name}
          onConfirm={() => handleDelete(deleteTarget)}
          onCancel={() => setDeleteTarget(null)}
          loading={deleting}
        />
      )}
    </div>
  );
}
