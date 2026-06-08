'use client';

import React, { useEffect, useState } from 'react';
import { ApiError, apiClient } from '../lib/api';
import { UsageOverviewItem } from '../lib/types';
import { AlertTriangle, CheckCircle2, DollarSign, RefreshCw, BarChart2, Cpu } from 'lucide-react';

const FALLBACK_MONTHLY_SPEND_LIMIT_USD = 10;
const MONTHLY_TOKEN_LIMIT = 200_000_000;
const WARNING_THRESHOLD = 80;
const DANGER_THRESHOLD = 100;

export default function UsageView() {
  const [usage, setUsage] = useState<UsageOverviewItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState('');
  const [dataUnavailable, setDataUnavailable] = useState(false);

  const fetchUsage = async () => {
    try {
      setLoading(true);
      setErrorMsg('');
      setDataUnavailable(false);
      const items = await apiClient.listUsageOverview();
      setUsage(Array.isArray(items) ? items as UsageOverviewItem[] : []);
    } catch (err) {
      console.error('Error fetching usage overview', err);
      setUsage([]);
      setDataUnavailable(true);
      if (err instanceof ApiError && err.status === 503) {
        setErrorMsg('Usage no esta disponible porque la base de datos esta desconectada.');
      } else {
        setErrorMsg('No se pudo cargar el consumo de modelos.');
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsage();
  }, []);

  const totalSpend = usage.reduce((sum, item) => sum + Number(item.totalCostUSD || 0), 0);
  const totalInput = usage.reduce((sum, item) => sum + Number(item.totalInputTokens || 0), 0);
  const totalOutput = usage.reduce((sum, item) => sum + Number(item.totalOutputTokens || 0), 0);
  const totalCached = usage.reduce((sum, item) => sum + Number(item.totalCachedTokens || 0), 0);
  const totalRequests = usage.reduce((sum, item) => sum + Number(item.totalRequests || 0), 0);
  const totalTokens = totalInput + totalOutput;
  const configuredProjectBudget = usage.reduce((projects, item) => {
    const budget = Number(item.projectMonthlyBudgetUSD || 0);
    if (budget > 0 && !projects.has(item.projectId)) {
      projects.set(item.projectId, budget);
    }
    return projects;
  }, new Map<string, number>());
  const configuredBudgetTotal = Array.from(configuredProjectBudget.values()).reduce((sum, budget) => sum + budget, 0);
  const effectiveSpendLimit = configuredBudgetTotal > 0 ? configuredBudgetTotal : FALLBACK_MONTHLY_SPEND_LIMIT_USD;
  const spendPercent = Math.min(100, (totalSpend / effectiveSpendLimit) * 100);
  const tokenPercent = Math.min(100, (totalTokens / MONTHLY_TOKEN_LIMIT) * 100);
  const hasEstimatedCosts = usage.some(item => item.costPrecision === 'estimated');
  const hasUnreportedCosts = usage.some(item => item.costPrecision === 'not_reported');
  const budgetAlertLevel = spendPercent >= DANGER_THRESHOLD ? 'danger' : spendPercent >= WARNING_THRESHOLD ? 'warning' : 'ok';
  const projectUsage = usage.reduce((projects, item) => {
    const key = item.projectId || 'global';
    const current = projects.get(key) || {
      projectId: key,
      projectName: item.projectName || key,
      budget: 0,
      cost: 0,
      requests: 0,
      tokens: 0,
    };
    current.budget = Math.max(current.budget, Number(item.projectMonthlyBudgetUSD || 0));
    current.cost += Number(item.totalCostUSD || 0);
    current.requests += Number(item.totalRequests || 0);
    current.tokens += Number(item.totalInputTokens || 0) + Number(item.totalOutputTokens || 0);
    projects.set(key, current);
    return projects;
  }, new Map<string, { projectId: string; projectName: string; budget: number; cost: number; requests: number; tokens: number }>());
  const projectRows = Array.from(projectUsage.values()).sort((a, b) => b.cost - a.cost);

  if (loading) {
    return (
      <div className="flex h-[55vh] items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <RefreshCw className="animate-spin text-primary" size={36} />
          <p className="text-sm text-muted-foreground">Cargando consumo de modelos...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {dataUnavailable && (
        <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-xs text-amber-100">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex gap-3">
              <AlertTriangle size={18} className="mt-0.5 shrink-0 text-amber-300" />
              <div>
                <p className="font-bold text-amber-200">Usage no disponible</p>
                <p className="mt-1 text-amber-100/80">
                  {errorMsg || 'Tokens, costos y presupuestos dependen de la base SQLite local. BattOS puede seguir mostrando telemetria mientras se recupera la DB.'}
                </p>
              </div>
            </div>
            <button
              onClick={fetchUsage}
              className="inline-flex items-center justify-center gap-1.5 rounded border border-amber-400/30 bg-black/20 px-3 py-1.5 font-semibold text-amber-100 hover:bg-black/30"
            >
              <RefreshCw size={12} /> Reintentar
            </button>
          </div>
        </div>
      )}

      {!dataUnavailable && usage.length > 0 && (
        <div className={`rounded-xl border p-4 text-xs ${
          budgetAlertLevel === 'danger'
            ? 'border-rose-500/30 bg-rose-500/10 text-rose-100'
            : budgetAlertLevel === 'warning'
              ? 'border-amber-500/30 bg-amber-500/10 text-amber-100'
              : 'border-emerald-500/20 bg-emerald-500/10 text-emerald-100'
        }`}>
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="flex gap-3">
              {budgetAlertLevel === 'ok' ? (
                <CheckCircle2 size={18} className="mt-0.5 shrink-0 text-emerald-300" />
              ) : (
                <AlertTriangle size={18} className="mt-0.5 shrink-0 text-amber-300" />
              )}
              <div>
                <p className="font-bold">
                  {budgetAlertLevel === 'danger'
                    ? 'Budget excedido'
                    : budgetAlertLevel === 'warning'
                      ? 'Budget cerca del limite'
                      : 'Budget bajo control'}
                </p>
                <p className="mt-1 opacity-80">
                  BattOS esta usando {spendPercent.toFixed(1)}% del limite mensual {configuredBudgetTotal > 0 ? 'configurado por proyectos' : 'estimado localmente'}.
                </p>
              </div>
            </div>
            {(hasEstimatedCosts || hasUnreportedCosts) && (
              <span className="rounded-full border border-current/20 px-3 py-1 text-[10px] font-bold uppercase tracking-wide opacity-80">
                {hasUnreportedCosts ? 'costos parciales' : 'costos estimados'}
              </span>
            )}
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="glass-panel rounded-xl p-4 border border-gray-800">
          <p className="text-xs uppercase tracking-wider text-muted-foreground">Costo mensual</p>
          <div className="mt-2 flex items-end justify-between gap-3">
            <span className="text-2xl font-bold text-white">${totalSpend.toFixed(4)}</span>
            <DollarSign size={22} className="text-primary" />
          </div>
          <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-gray-800">
            <div className="h-full bg-primary" style={{ width: `${spendPercent}%` }} />
          </div>
          <p className="mt-1 text-[10px] text-muted-foreground">
            {spendPercent.toFixed(1)}% de ${effectiveSpendLimit.toFixed(2)}
          </p>
          <p className="mt-1 text-[10px] text-muted-foreground">
            {configuredBudgetTotal > 0 ? 'Budget de proyectos' : 'Fallback local sin budgets'}
          </p>
        </div>

        <div className="glass-panel rounded-xl p-4 border border-gray-800">
          <p className="text-xs uppercase tracking-wider text-muted-foreground">Tokens</p>
          <div className="mt-2 flex items-end justify-between gap-3">
            <span className="text-2xl font-bold text-white">{(totalTokens / 1000).toFixed(1)}k</span>
            <BarChart2 size={22} className="text-indigo-400" />
          </div>
          <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-gray-800">
            <div className="h-full bg-indigo-500" style={{ width: `${tokenPercent}%` }} />
          </div>
          <p className="mt-1 text-[10px] text-muted-foreground">
            {tokenPercent.toFixed(4)}% de {(MONTHLY_TOKEN_LIMIT / 1_000_000).toFixed(0)}M
          </p>
        </div>

        <div className="glass-panel rounded-xl p-4 border border-gray-800">
          <p className="text-xs uppercase tracking-wider text-muted-foreground">Requests</p>
          <div className="mt-2 flex items-end justify-between gap-3">
            <span className="text-2xl font-bold text-white">{totalRequests}</span>
            <Cpu size={22} className="text-emerald-400" />
          </div>
          <p className="mt-3 text-[10px] text-muted-foreground">Solicitudes de modelo registradas</p>
        </div>

        <div className="glass-panel rounded-xl p-4 border border-gray-800">
          <p className="text-xs uppercase tracking-wider text-muted-foreground">Cache tokens</p>
          <div className="mt-2 flex items-end justify-between gap-3">
            <span className="text-2xl font-bold text-white">{(totalCached / 1000).toFixed(1)}k</span>
            <RefreshCw size={22} className="text-cyan-400" />
          </div>
          <p className="mt-3 text-[10px] text-muted-foreground">Ahorro potencial por contexto cacheado</p>
        </div>
      </div>

      <div className="glass-panel rounded-xl border border-gray-800 p-5 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-base font-bold text-white flex items-center gap-2">
            <DollarSign size={18} className="text-primary" />
            Budgets por proyecto
          </h3>
          <span className="text-[10px] uppercase tracking-wider text-muted-foreground">
            Umbral alerta {WARNING_THRESHOLD}%
          </span>
        </div>

        {projectRows.length === 0 ? (
          <div className="flex h-32 items-center justify-center rounded-xl border border-gray-800/60 bg-gray-950/40 text-sm text-muted-foreground">
            No hay proyectos con consumo registrado todavia.
          </div>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {projectRows.map(project => {
              const projectLimit = project.budget > 0 ? project.budget : FALLBACK_MONTHLY_SPEND_LIMIT_USD;
              const pct = Math.min(100, (project.cost / projectLimit) * 100);
              const tone = pct >= DANGER_THRESHOLD ? 'bg-rose-500' : pct >= WARNING_THRESHOLD ? 'bg-amber-400' : 'bg-emerald-400';
              return (
                <div key={project.projectId} className="rounded-xl border border-gray-800 bg-black/30 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="font-bold text-white">{project.projectName || project.projectId}</p>
                      <p className="mt-1 text-[10px] text-muted-foreground">{project.projectId}</p>
                    </div>
                    <span className="font-mono text-xs text-emerald-300">${project.cost.toFixed(4)}</span>
                  </div>
                  <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-gray-800">
                    <div className={`h-full ${tone}`} style={{ width: `${pct}%` }} />
                  </div>
                  <div className="mt-2 flex justify-between text-[10px] text-muted-foreground">
                    <span>{pct.toFixed(1)}%</span>
                    <span>{project.budget > 0 ? `$${project.budget.toFixed(2)} budget` : 'sin budget configurado'}</span>
                  </div>
                  <div className="mt-3 flex justify-between text-[10px] text-muted-foreground">
                    <span>{project.requests} requests</span>
                    <span>{(project.tokens / 1000).toFixed(1)}k tokens</span>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <div className="glass-panel rounded-xl border border-gray-800 p-5 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-base font-bold text-white flex items-center gap-2">
            <BarChart2 size={18} className="text-primary" />
            Distribucion por proyecto, agente y modelo
          </h3>
          <button
            onClick={fetchUsage}
            className="inline-flex items-center gap-1.5 rounded border border-gray-800 bg-gray-900 px-3 py-1.5 text-xs font-semibold text-gray-200 hover:bg-gray-800"
          >
            <RefreshCw size={12} /> Refrescar
          </button>
        </div>

        {usage.length === 0 ? (
          <div className="flex h-44 items-center justify-center rounded-xl border border-gray-800/60 bg-gray-950/40 text-sm text-muted-foreground">
            {dataUnavailable ? 'Usage queda pausado hasta recuperar la base SQLite local.' : 'No hay eventos de consumo registrados todavia.'}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-gray-800 text-muted-foreground">
                  <th className="py-2.5">Proyecto</th>
                  <th className="py-2.5">Agente</th>
                  <th className="py-2.5">Proveedor</th>
                  <th className="py-2.5">Modelo</th>
                  <th className="py-2.5">Precision</th>
                  <th className="py-2.5 text-right">Requests</th>
                  <th className="py-2.5 text-right">Input</th>
                  <th className="py-2.5 text-right">Output</th>
                  <th className="py-2.5 text-right">Costo</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800 text-gray-200">
                {usage.map((item, idx) => (
                  <tr key={`${item.projectId}-${item.agentId}-${item.modelId}-${idx}`} className="hover:bg-gray-900/30">
                    <td className="py-3">
                      <p className="font-semibold text-white">{item.projectName || item.projectId || 'Global'}</p>
                      <p className="text-[10px] text-muted-foreground">{item.projectId || 'global'}</p>
                    </td>
                    <td className="py-3">{item.agentId || 'N/A'}</td>
                    <td className="py-3 text-muted-foreground">{item.providerId || 'N/A'}</td>
                    <td className="py-3 text-gray-300">{item.modelId || 'N/A'}</td>
                    <td className="py-3">
                      <span className={`rounded-full px-2 py-0.5 text-[10px] font-bold uppercase ${
                        item.costPrecision === 'exact'
                          ? 'bg-emerald-500/10 text-emerald-300'
                          : item.costPrecision === 'estimated'
                            ? 'bg-amber-500/10 text-amber-300'
                            : 'bg-gray-800 text-muted-foreground'
                      }`}>
                        {item.costPrecision || 'not_reported'}
                      </span>
                    </td>
                    <td className="py-3 text-right">{item.totalRequests}</td>
                    <td className="py-3 text-right">{item.totalInputTokens.toLocaleString()}</td>
                    <td className="py-3 text-right">{item.totalOutputTokens.toLocaleString()}</td>
                    <td className="py-3 text-right font-mono text-emerald-400">${Number(item.totalCostUSD).toFixed(5)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
