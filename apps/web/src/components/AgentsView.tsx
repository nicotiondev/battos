'use client';

import React, { useEffect, useMemo, useState } from 'react';
import { ApiError, apiClient } from '../lib/api';
import { Agent, AgentRuntime } from '../lib/types';
import {
  AlertTriangle,
  Bot,
  CheckCircle2,
  Cpu,
  Plus,
  RefreshCw,
  ShieldCheck,
  Terminal,
  X,
} from 'lucide-react';

const EXECUTABLE_V01_RUNTIMES = new Set(['claude-code', 'codex', 'sandbox-smoke', 'sandbox-memory-smoke']);

type AgentForm = {
  slug: string;
  name: string;
  role: string;
  description: string;
  runtimeId: string;
  systemPrompt: string;
  riskLevel: 'low' | 'medium' | 'high';
  status: 'active' | 'paused' | 'archived';
};

function defaultForm(runtimeId = ''): AgentForm {
  return {
    slug: '',
    name: '',
    role: '',
    description: '',
    runtimeId,
    systemPrompt: '',
    riskLevel: 'medium',
    status: 'active',
  };
}

function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

function runtimeStatusTone(status?: string): string {
  if (status === 'configured' || status === 'detected') {
    return 'border-emerald-500/20 bg-emerald-500/10 text-emerald-300';
  }
  if (status === 'blocked') {
    return 'border-amber-500/20 bg-amber-500/10 text-amber-300';
  }
  return 'border-gray-700 bg-gray-900 text-muted-foreground';
}

function riskTone(risk?: string): string {
  if (risk === 'high') return 'border-rose-500/20 bg-rose-500/10 text-rose-300';
  if (risk === 'low') return 'border-emerald-500/20 bg-emerald-500/10 text-emerald-300';
  return 'border-amber-500/20 bg-amber-500/10 text-amber-300';
}

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

function runtimeLabel(runtime?: AgentRuntime): string {
  if (!runtime) return 'Runtime no encontrado';
  const command = runtime.command ? ` / ${runtime.command}` : '';
  return `${runtime.name}${command}`;
}

export default function AgentsView() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [runtimes, setRuntimes] = useState<AgentRuntime[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState<AgentForm>(defaultForm());
  const [errorMsg, setErrorMsg] = useState('');
  const [successMsg, setSuccessMsg] = useState('');
  const [dataUnavailable, setDataUnavailable] = useState(false);
  const [saving, setSaving] = useState(false);

  const runtimeById = useMemo(() => new Map(runtimes.map(runtime => [runtime.id, runtime])), [runtimes]);
  const activeAgents = agents.filter(agent => agent.status === 'active').length;
  const configuredRuntimes = runtimes.filter(runtime => runtime.status === 'configured' || runtime.status === 'detected').length;
  const executableRuntimes = runtimes.filter(runtime => EXECUTABLE_V01_RUNTIMES.has(runtime.id));

  const fetchData = async () => {
    try {
      setLoading(true);
      setErrorMsg('');
      setDataUnavailable(false);
      const [agentItems, runtimeItems] = await Promise.all([
        apiClient.listAgents(),
        apiClient.listRuntimeAdapters(),
      ]);
      const nextAgents = Array.isArray(agentItems) ? agentItems as Agent[] : [];
      const nextRuntimes = Array.isArray(runtimeItems) ? runtimeItems as AgentRuntime[] : [];
      setAgents(nextAgents.sort((a, b) => a.name.localeCompare(b.name)));
      setRuntimes(nextRuntimes);
      setForm(prev => ({
        ...prev,
        runtimeId: prev.runtimeId || nextRuntimes.find(runtime => EXECUTABLE_V01_RUNTIMES.has(runtime.id))?.id || nextRuntimes[0]?.id || '',
      }));
    } catch (err) {
      console.error('Error fetching agents registry', err);
      setAgents([]);
      setRuntimes([]);
      setDataUnavailable(true);
      if (err instanceof ApiError && err.status === 503) {
        setErrorMsg('Agents Registry no esta disponible porque la base de datos esta desconectada.');
      } else {
        setErrorMsg('No se pudieron cargar agentes y runtimes.');
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleNameChange = (name: string) => {
    setForm(prev => ({
      ...prev,
      name,
      slug: prev.slug || slugify(name),
    }));
  };

  const handleCreateAgent = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const slug = slugify(form.slug);
    if (!slug || !form.name.trim() || !form.runtimeId) {
      setErrorMsg('Completa slug, nombre y runtime antes de crear el agente.');
      return;
    }

    try {
      setSaving(true);
      setErrorMsg('');
      setSuccessMsg('');
      const created = await apiClient.createAgent({
        slug,
        name: form.name.trim(),
        role: form.role.trim() || undefined,
        description: form.description.trim() || undefined,
        runtimeId: form.runtimeId,
        systemPrompt: form.systemPrompt.trim() || undefined,
        riskLevel: form.riskLevel,
        status: form.status,
      }) as Agent;
      setAgents(prev => [...prev, created].sort((a, b) => a.name.localeCompare(b.name)));
      setForm(defaultForm(form.runtimeId));
      setShowCreate(false);
      setSuccessMsg(`Agente ${created.name} creado. Ya puede asignarse a tasks o runs supervisados.`);
    } catch (err) {
      setErrorMsg(errorMessage(err, 'No se pudo crear el agente.'));
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex h-[55vh] items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <RefreshCw className="animate-spin text-primary" size={36} />
          <p className="text-sm text-muted-foreground">Cargando agentes y runtimes...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <section className="glass-panel rounded-2xl border border-gray-800 p-6">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="flex items-center gap-2 text-primary">
              <Bot size={18} />
              <p className="text-xs font-bold uppercase tracking-widest">Agents Registry</p>
            </div>
            <h2 className="mt-2 text-2xl font-bold text-white">Agentes operativos de BattOS</h2>
            <p className="mt-2 max-w-3xl text-sm text-muted-foreground">
              Crea identidades de agente con runtime, rol y prompt base. En v0.1 un agente no ejecuta nada solo:
              Control Room propone un run, exige aprobacion humana y luego el worker lo corre en sandbox.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={fetchData}
              className="inline-flex items-center justify-center gap-2 rounded-lg border border-gray-800 bg-gray-950 px-4 py-2 text-xs font-bold uppercase tracking-wide text-gray-200 hover:border-primary/50 hover:text-primary"
            >
              <RefreshCw size={14} /> Refrescar
            </button>
            <button
              onClick={() => setShowCreate(true)}
              disabled={dataUnavailable || runtimes.length === 0}
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2 text-xs font-bold uppercase tracking-wide text-primary-foreground hover:bg-yellow-400 disabled:cursor-not-allowed disabled:opacity-40"
            >
              <Plus size={14} /> Crear agente
            </button>
          </div>
        </div>
      </section>

      {(dataUnavailable || errorMsg || successMsg) && (
        <div className={`rounded-xl border p-4 text-xs ${
          successMsg && !errorMsg
            ? 'border-emerald-500/20 bg-emerald-500/10 text-emerald-100'
            : 'border-amber-500/30 bg-amber-500/10 text-amber-100'
        }`}>
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="flex gap-3">
              {successMsg && !errorMsg ? (
                <CheckCircle2 size={18} className="mt-0.5 shrink-0 text-emerald-300" />
              ) : (
                <AlertTriangle size={18} className="mt-0.5 shrink-0 text-amber-300" />
              )}
              <div>
                <p className="font-bold">{successMsg && !errorMsg ? 'Agents Registry actualizado' : 'Agents Registry requiere atencion'}</p>
                <p className="mt-1 opacity-80">
                  {errorMsg || successMsg || 'Agentes y runtimes dependen de Postgres. Recupera la DB para crear o listar agentes.'}
                </p>
              </div>
            </div>
            {errorMsg && (
              <button onClick={() => setErrorMsg('')} className="text-amber-100/80 hover:text-white">
                <X size={14} />
              </button>
            )}
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <div className="glass-panel rounded-xl border border-gray-800 p-4">
          <p className="text-xs uppercase tracking-wider text-muted-foreground">Agentes</p>
          <div className="mt-2 flex items-end justify-between gap-3">
            <span className="text-2xl font-bold text-white">{agents.length}</span>
            <Bot size={22} className="text-primary" />
          </div>
          <p className="mt-2 text-[10px] text-muted-foreground">{activeAgents} activos</p>
        </div>
        <div className="glass-panel rounded-xl border border-gray-800 p-4">
          <p className="text-xs uppercase tracking-wider text-muted-foreground">Runtimes</p>
          <div className="mt-2 flex items-end justify-between gap-3">
            <span className="text-2xl font-bold text-white">{runtimes.length}</span>
            <Cpu size={22} className="text-indigo-400" />
          </div>
          <p className="mt-2 text-[10px] text-muted-foreground">{configuredRuntimes} detectados/configurados</p>
        </div>
        <div className="glass-panel rounded-xl border border-gray-800 p-4">
          <p className="text-xs uppercase tracking-wider text-muted-foreground">Ejecutables v0.1</p>
          <div className="mt-2 flex items-end justify-between gap-3">
            <span className="text-2xl font-bold text-white">{executableRuntimes.length}</span>
            <Terminal size={22} className="text-emerald-400" />
          </div>
          <p className="mt-2 text-[10px] text-muted-foreground">Claude Code, Codex y smokes internos</p>
        </div>
        <div className="glass-panel rounded-xl border border-gray-800 p-4">
          <p className="text-xs uppercase tracking-wider text-muted-foreground">Guardrail</p>
          <div className="mt-2 flex items-end justify-between gap-3">
            <span className="text-lg font-bold text-white">HITL</span>
            <ShieldCheck size={22} className="text-amber-300" />
          </div>
          <p className="mt-2 text-[10px] text-muted-foreground">Crear agente no aprueba ejecucion</p>
        </div>
      </div>

      <div className="grid gap-6 xl:grid-cols-3">
        <section className="glass-panel rounded-2xl border border-gray-800 p-5 xl:col-span-2">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-base font-bold text-white">Agentes registrados</h3>
            <span className="text-[10px] uppercase tracking-wider text-muted-foreground">
              Identidad + runtime + memoria futura
            </span>
          </div>

          {agents.length === 0 ? (
            <div className="flex h-56 flex-col items-center justify-center rounded-xl border border-gray-800/60 bg-gray-950/40 p-6 text-center">
              <Bot size={38} className="text-muted-foreground" />
              <h4 className="mt-4 text-sm font-bold text-white">Aun no hay agentes</h4>
              <p className="mt-2 max-w-md text-xs text-muted-foreground">
                Crea un agente para poder asignarlo a tareas y proponer runs en Control Room.
              </p>
              <button
                onClick={() => setShowCreate(true)}
                disabled={dataUnavailable || runtimes.length === 0}
                className="mt-4 inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-xs font-bold uppercase tracking-wide text-primary-foreground hover:bg-yellow-400 disabled:cursor-not-allowed disabled:opacity-40"
              >
                <Plus size={14} /> Crear primer agente
              </button>
            </div>
          ) : (
            <div className="grid gap-3 md:grid-cols-2">
              {agents.map(agent => {
                const runtime = agent.runtimeId ? runtimeById.get(agent.runtimeId) : undefined;
                const executable = Boolean(agent.runtimeId && EXECUTABLE_V01_RUNTIMES.has(agent.runtimeId));
                return (
                  <article key={agent.id} className="rounded-xl border border-gray-800 bg-black/30 p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <div className="flex flex-wrap items-center gap-2">
                          <h4 className="font-bold text-white">{agent.name}</h4>
                          <span className={`rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase ${riskTone(agent.riskLevel)}`}>
                            {agent.riskLevel || 'medium'}
                          </span>
                        </div>
                        <p className="mt-1 font-mono text-[10px] text-muted-foreground">{agent.slug || agent.id}</p>
                      </div>
                      <span className={`rounded-full px-2 py-0.5 text-[10px] font-bold uppercase ${
                        agent.status === 'active'
                          ? 'bg-emerald-500/10 text-emerald-300'
                          : 'bg-gray-800 text-muted-foreground'
                      }`}>
                        {agent.status}
                      </span>
                    </div>
                    <p className="mt-3 text-xs leading-relaxed text-muted-foreground">
                      {agent.description || agent.role || 'Sin descripcion. Este agente puede recibir un prompt base y permisos finos en la siguiente iteracion.'}
                    </p>
                    <div className="mt-4 rounded-lg border border-gray-800 bg-gray-950/70 p-3 text-xs">
                      <div className="flex items-center justify-between gap-3">
                        <span className="font-semibold text-gray-200">{runtimeLabel(runtime)}</span>
                        <span className={`rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase ${runtimeStatusTone(runtime?.status)}`}>
                          {runtime?.status || 'missing'}
                        </span>
                      </div>
                      <p className="mt-2 text-[10px] text-muted-foreground">
                        {executable
                          ? 'Runtime aprobado para runs supervisados en v0.1.'
                          : 'Runtime registrado, pero no ejecutable automaticamente en v0.1.'}
                      </p>
                    </div>
                  </article>
                );
              })}
            </div>
          )}
        </section>

        <section className="glass-panel rounded-2xl border border-gray-800 p-5">
          <div className="mb-4 flex items-center gap-2">
            <Terminal size={16} className="text-primary" />
            <h3 className="text-sm font-bold text-white">Runtimes disponibles</h3>
          </div>
          <div className="space-y-3">
            {runtimes.length === 0 ? (
              <div className="rounded-xl border border-gray-800 bg-black/30 p-4 text-xs text-muted-foreground">
                No hay runtimes registrados. Revisa migraciones/Postgres.
              </div>
            ) : (
              runtimes.map(runtime => {
                const executable = EXECUTABLE_V01_RUNTIMES.has(runtime.id);
                return (
                  <div key={runtime.id} className="rounded-xl border border-gray-800 bg-black/30 p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-xs font-bold text-white">{runtime.name}</p>
                        <p className="mt-1 font-mono text-[10px] text-muted-foreground">{runtime.id}</p>
                      </div>
                      <span className={`rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase ${runtimeStatusTone(runtime.status)}`}>
                        {runtime.status}
                      </span>
                    </div>
                    <div className="mt-3 flex flex-wrap gap-2 text-[10px]">
                      {runtime.command && (
                        <span className="rounded-full bg-gray-900 px-2 py-0.5 text-gray-300">{runtime.command}</span>
                      )}
                      {runtime.requiresAuth && (
                        <span className="rounded-full bg-amber-500/10 px-2 py-0.5 text-amber-300">auth</span>
                      )}
                      <span className={`rounded-full px-2 py-0.5 ${
                        executable ? 'bg-emerald-500/10 text-emerald-300' : 'bg-gray-900 text-muted-foreground'
                      }`}>
                        {executable ? 'ejecutable v0.1' : 'catalogo/futuro'}
                      </span>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </section>
      </div>

      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="glass-panel max-h-[92vh] w-full max-w-2xl overflow-y-auto rounded-xl p-6">
            <div className="mb-4 flex items-center justify-between border-b border-gray-800 pb-3">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <Bot size={16} className="text-primary" /> Crear agente
              </h3>
              <button onClick={() => setShowCreate(false)} className="text-muted-foreground hover:text-white">
                <X size={16} />
              </button>
            </div>

            <form onSubmit={handleCreateAgent} className="space-y-4">
              <div className="grid gap-4 md:grid-cols-2">
                <div>
                  <label className="mb-1 block text-[10px] font-semibold uppercase text-muted-foreground">Nombre</label>
                  <input
                    value={form.name}
                    onChange={(event) => handleNameChange(event.target.value)}
                    placeholder="Builder Web"
                    required
                    className="w-full rounded border border-gray-800 bg-gray-950 p-2 text-xs text-white outline-none focus:border-primary"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-[10px] font-semibold uppercase text-muted-foreground">Slug</label>
                  <input
                    value={form.slug}
                    onChange={(event) => setForm(prev => ({ ...prev, slug: slugify(event.target.value) }))}
                    placeholder="builder-web"
                    required
                    className="w-full rounded border border-gray-800 bg-gray-950 p-2 font-mono text-xs text-white outline-none focus:border-primary"
                  />
                </div>
              </div>

              <div className="grid gap-4 md:grid-cols-3">
                <div>
                  <label className="mb-1 block text-[10px] font-semibold uppercase text-muted-foreground">Runtime</label>
                  <select
                    value={form.runtimeId}
                    onChange={(event) => setForm(prev => ({ ...prev, runtimeId: event.target.value }))}
                    required
                    className="w-full rounded border border-gray-800 bg-gray-950 p-2 text-xs text-white outline-none focus:border-primary"
                  >
                    <option value="">Selecciona runtime</option>
                    {runtimes.map(runtime => (
                      <option key={runtime.id} value={runtime.id}>
                        {runtime.name} ({runtime.status})
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="mb-1 block text-[10px] font-semibold uppercase text-muted-foreground">Riesgo</label>
                  <select
                    value={form.riskLevel}
                    onChange={(event) => setForm(prev => ({ ...prev, riskLevel: event.target.value as AgentForm['riskLevel'] }))}
                    className="w-full rounded border border-gray-800 bg-gray-950 p-2 text-xs text-white outline-none focus:border-primary"
                  >
                    <option value="low">low</option>
                    <option value="medium">medium</option>
                    <option value="high">high</option>
                  </select>
                </div>
                <div>
                  <label className="mb-1 block text-[10px] font-semibold uppercase text-muted-foreground">Estado</label>
                  <select
                    value={form.status}
                    onChange={(event) => setForm(prev => ({ ...prev, status: event.target.value as AgentForm['status'] }))}
                    className="w-full rounded border border-gray-800 bg-gray-950 p-2 text-xs text-white outline-none focus:border-primary"
                  >
                    <option value="active">active</option>
                    <option value="paused">paused</option>
                    <option value="archived">archived</option>
                  </select>
                </div>
              </div>

              <div>
                <label className="mb-1 block text-[10px] font-semibold uppercase text-muted-foreground">Rol opcional</label>
                <input
                  value={form.role}
                  onChange={(event) => setForm(prev => ({ ...prev, role: event.target.value }))}
                  placeholder="web_builder, analyst, reviewer..."
                  className="w-full rounded border border-gray-800 bg-gray-950 p-2 text-xs text-white outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="mb-1 block text-[10px] font-semibold uppercase text-muted-foreground">Descripcion</label>
                <textarea
                  value={form.description}
                  onChange={(event) => setForm(prev => ({ ...prev, description: event.target.value }))}
                  placeholder="Que hace este agente y cuando conviene usarlo."
                  rows={3}
                  className="w-full rounded border border-gray-800 bg-gray-950 p-2 text-xs text-white outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="mb-1 block text-[10px] font-semibold uppercase text-muted-foreground">Prompt base opcional</label>
                <textarea
                  value={form.systemPrompt}
                  onChange={(event) => setForm(prev => ({ ...prev, systemPrompt: event.target.value }))}
                  placeholder="Instrucciones estables para este agente. No pegues secretos."
                  rows={5}
                  className="w-full rounded border border-gray-800 bg-gray-950 p-2 font-mono text-xs text-white outline-none focus:border-primary"
                />
                <p className="mt-1 text-[10px] text-muted-foreground">
                  El prompt base se guarda como configuracion del agente. La ejecucion real igual requiere aprobacion en Control Room.
                </p>
              </div>

              <div className="rounded-xl border border-amber-500/20 bg-amber-500/10 p-3 text-xs text-amber-100">
                Detectar o seleccionar un runtime no concede ejecucion automatica. Los runs v0.1 siguen pasando por sandbox, logs y aprobaciones HITL.
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreate(false)}
                  className="rounded bg-gray-900 px-3 py-1.5 text-xs text-white hover:bg-gray-800"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="rounded bg-primary px-3 py-1.5 text-xs font-semibold text-primary-foreground hover:bg-yellow-400 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {saving ? 'Creando...' : 'Crear agente'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
