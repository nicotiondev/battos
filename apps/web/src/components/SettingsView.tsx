'use client';

import React, { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  Database,
  Key,
  Lock,
  RefreshCw,
  Server,
  ShieldCheck,
  SlidersHorizontal,
  WifiOff,
} from 'lucide-react';
import { clearApiToken, getApiBaseUrl, getApiToken, setApiToken } from '../lib/api';
import { StatusResponse, SubsystemHealth } from '../lib/types';

interface SettingsViewProps {
  metrics: StatusResponse | null;
  apiOnline: boolean;
  onRefresh: () => void;
}

function statusTone(status?: string) {
  if (status === 'ok') return 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20';
  if (status === 'degraded' || status === 'unknown') return 'text-amber-300 bg-amber-500/10 border-amber-500/20';
  return 'text-rose-300 bg-rose-500/10 border-rose-500/20';
}

function StatusPill({ status }: { status?: string }) {
  return (
    <span className={`rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide ${statusTone(status)}`}>
      {status || 'offline'}
    </span>
  );
}

function subsystemByName(subsystems: SubsystemHealth[] | undefined, name: string) {
  return subsystems?.find(item => item.name === name);
}

export default function SettingsView({ metrics, apiOnline, onRefresh }: SettingsViewProps) {
  const [hasToken, setHasToken] = useState(false);
  const [tokenInput, setTokenInput] = useState('');
  const [tokenCleared, setTokenCleared] = useState(false);
  const [tokenSaved, setTokenSaved] = useState(false);
  const apiBaseUrl = getApiBaseUrl();

  useEffect(() => {
    setHasToken(Boolean(getApiToken()));
  }, []);

  const database = subsystemByName(metrics?.subsystems, 'database');
  const memory = subsystemByName(metrics?.subsystems, 'memory');
  const config = subsystemByName(metrics?.subsystems, 'config');
  const sysmetrics = subsystemByName(metrics?.subsystems, 'sysmetrics');

  const systemCards = useMemo(() => [
    { label: 'API', status: apiOnline ? 'ok' : 'down', detail: apiOnline ? 'BattOS API responde en local.' : 'No hay conexion con la API.' },
    { label: 'Config', status: config?.status || 'unknown', detail: config?.detail || 'Sin detalle de configuracion.' },
    { label: 'Database', status: database?.status || 'unknown', detail: database?.detail || 'SQLite local no reporta estado.' },
    { label: 'Memory Core', status: memory?.status || 'unknown', detail: memory?.detail || 'SQLite/FTS5 no reporta estado.' },
    { label: 'Sysmetrics', status: sysmetrics?.status || 'unknown', detail: sysmetrics?.detail || 'Metricas de host sin detalle.' },
  ], [apiOnline, config, database, memory, sysmetrics]);

  const handleClearToken = () => {
    clearApiToken();
    setHasToken(false);
    setTokenInput('');
    setTokenCleared(true);
    setTokenSaved(false);
    onRefresh();
  };

  const handleSaveToken = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = tokenInput.trim();
    if (!trimmed) return;
    setApiToken(trimmed);
    setHasToken(true);
    setTokenInput('');
    setTokenSaved(true);
    setTokenCleared(false);
    onRefresh();
  };

  return (
    <div className="space-y-6">
      <section className="glass-panel rounded-2xl border border-gray-800 p-6">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="flex items-center gap-2 text-primary">
              <SlidersHorizontal size={18} />
              <p className="text-xs font-bold uppercase tracking-widest">Settings</p>
            </div>
            <h2 className="mt-2 text-2xl font-bold text-white">Configuracion operativa</h2>
            <p className="mt-2 max-w-3xl text-sm text-muted-foreground">
              Panel de diagnostico para revisar conexion, salud del sistema y guardrails de seguridad sin exponer secretos.
            </p>
          </div>
          <button
            onClick={onRefresh}
            className="inline-flex items-center justify-center gap-2 rounded-lg border border-gray-800 bg-gray-950 px-4 py-2 text-xs font-bold uppercase tracking-wide text-gray-200 hover:border-primary/50 hover:text-primary"
          >
            <RefreshCw size={14} /> Refrescar
          </button>
        </div>
      </section>

      <div className="grid gap-4 lg:grid-cols-3">
        <section className="glass-panel rounded-2xl border border-gray-800 p-5 lg:col-span-2">
          <div className="mb-4 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Server size={16} className="text-primary" />
              <h3 className="text-sm font-bold text-white">Estado del OS</h3>
            </div>
            <StatusPill status={apiOnline ? metrics?.overall || 'ok' : 'down'} />
          </div>

          <div className="grid gap-3 md:grid-cols-2">
            {systemCards.map(card => (
              <div key={card.label} className="rounded-xl border border-gray-800 bg-black/30 p-4">
                <div className="flex items-center justify-between gap-3">
                  <p className="text-xs font-bold text-white">{card.label}</p>
                  <StatusPill status={card.status} />
                </div>
                <p className="mt-2 text-xs leading-relaxed text-muted-foreground">{card.detail}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="glass-panel rounded-2xl border border-gray-800 p-5">
          <div className="mb-4 flex items-center gap-2">
            <Key size={16} className="text-primary" />
            <h3 className="text-sm font-bold text-white">Acceso local</h3>
          </div>

          <div className="space-y-3 text-xs">
            <div className="rounded-xl border border-gray-800 bg-black/30 p-4">
              <p className="text-muted-foreground">API URL</p>
              <p className="mt-1 break-all font-mono text-gray-100">{apiBaseUrl}</p>
            </div>

            <div className="rounded-xl border border-gray-800 bg-black/30 p-4">
              <p className="text-muted-foreground">Token en navegador</p>
              <div className="mt-2 flex items-center justify-between gap-3">
                <p className={hasToken ? 'text-emerald-300' : 'text-amber-300'}>
                  {hasToken ? 'Guardado localmente' : 'No guardado'}
                </p>
                <Lock size={14} className="text-muted-foreground" />
              </div>
              <p className="mt-2 text-muted-foreground">
                BattOS nunca muestra el valor del token en este panel.
              </p>
            </div>

            <form onSubmit={handleSaveToken} className="rounded-xl border border-gray-800 bg-black/30 p-4">
              <label htmlFor="settings-api-token" className="text-muted-foreground">
                Guardar o reemplazar token
              </label>
              <input
                id="settings-api-token"
                type="password"
                value={tokenInput}
                onChange={(event) => setTokenInput(event.target.value)}
                placeholder="BATTOS_API_TOKEN"
                className="mt-2 w-full rounded-lg border border-gray-800 bg-gray-950 px-3 py-2 font-mono text-xs text-white outline-none placeholder:text-gray-600 focus:border-primary/60"
              />
              <button
                type="submit"
                disabled={!tokenInput.trim()}
                className="mt-3 w-full rounded-lg bg-primary px-3 py-2 font-bold uppercase tracking-wide text-primary-foreground hover:bg-yellow-400 disabled:cursor-not-allowed disabled:opacity-40"
              >
                Guardar token local
              </button>
              <p className="mt-2 text-muted-foreground">
                Se guarda solo en este navegador y se usa como Bearer token para API y SSE.
              </p>
            </form>

            <button
              onClick={handleClearToken}
              disabled={!hasToken}
              className="w-full rounded-lg border border-gray-800 bg-gray-950 px-3 py-2 font-bold uppercase tracking-wide text-gray-200 hover:border-rose-500/40 hover:text-rose-300 disabled:cursor-not-allowed disabled:opacity-40"
            >
              Limpiar token local
            </button>
            {tokenSaved && (
              <p className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3 text-emerald-200">
                Token local guardado. BattOS intentara refrescar el estado usando ese acceso.
              </p>
            )}
            {tokenCleared && (
              <p className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-3 text-amber-200">
                Token local eliminado. Si la API exige auth, el proximo intento pedira acceso nuevamente.
              </p>
            )}
          </div>
        </section>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <section className="glass-panel rounded-2xl border border-gray-800 p-5">
          <div className="mb-4 flex items-center gap-2">
            <ShieldCheck size={16} className="text-emerald-400" />
            <h3 className="text-sm font-bold text-white">Guardrails v0.1</h3>
          </div>
          <div className="space-y-3">
            {[
              'Runs supervisados: ejecutar, red, commit y push requieren aprobacion humana.',
              'Los adapters Codex/Claude se ejecutan en contenedores efimeros, no como shell arbitraria en host.',
              'Los secretos se manejan por referencia y no se muestran en el dashboard.',
              'SSE se usa para streaming operacional; WebSockets queda fuera de v0.1.',
            ].map(item => (
              <div key={item} className="flex gap-3 rounded-xl border border-gray-800 bg-black/30 p-3 text-xs text-muted-foreground">
                <CheckCircle2 size={14} className="mt-0.5 shrink-0 text-emerald-400" />
                <span>{item}</span>
              </div>
            ))}
          </div>
        </section>

        <section className="glass-panel rounded-2xl border border-gray-800 p-5">
          <div className="mb-4 flex items-center gap-2">
            {apiOnline ? <Database size={16} className="text-primary" /> : <WifiOff size={16} className="text-rose-300" />}
            <h3 className="text-sm font-bold text-white">Modo operativo</h3>
          </div>
          <div className="space-y-3 text-xs text-muted-foreground">
            <p className="rounded-xl border border-gray-800 bg-black/30 p-4">
              En desarrollo local, el dashboard usa la base SQLite local en `data/battos.db`. Si la DB cae, las pantallas con datos operacionales muestran modo degradado hasta recuperarla.
            </p>
            {database && database.status !== 'ok' && (
              <div className="flex gap-3 rounded-xl border border-amber-500/20 bg-amber-500/10 p-4 text-amber-100">
                <AlertTriangle size={16} className="mt-0.5 shrink-0 text-amber-300" />
                <div>
                  <p className="font-bold">Base de datos no disponible</p>
                  <p className="mt-1 text-amber-100/80">{database.detail || 'Recupera la base SQLite local para habilitar Work Board, runs, usage completo y repositorios.'}</p>
                </div>
              </div>
            )}
            <p className="rounded-xl border border-gray-800 bg-black/30 p-4">
              En produccion, este panel debe evolucionar hacia configuracion modular, providers, limites, llaves por referencia y health checks por servicio.
            </p>
          </div>
        </section>
      </div>
    </div>
  );
}
