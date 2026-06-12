'use client';

import React, { useState, useEffect } from 'react';
import { apiClient } from '../lib/api';
import { Project, Agent, Skill, AgentRuntime, StatusResponse, UsageOverviewItem } from '../lib/types';
import { 
  Folder, Cpu, Award, Shield, Activity, DollarSign, BarChart2,
  CheckCircle, AlertTriangle, XCircle, RefreshCw, Zap, HardDrive
} from 'lucide-react';

interface DashboardViewProps {
  metrics: StatusResponse | null;
  // Navegación a otras vistas del shell al clickear las cards de resumen.
  onNavigate?: (tab: 'workboard' | 'agents' | 'controlroom' | 'usage') => void;
}

export default function DashboardView({ metrics, onNavigate }: DashboardViewProps) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [skills, setSkills] = useState<Skill[]>([]);
  const [runtimes, setRuntimes] = useState<AgentRuntime[]>([]);
  const [usage, setUsage] = useState<UsageOverviewItem[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchData = async () => {
    try {
      setLoading(true);
      const [projs, ags, sks, rts, usg] = await Promise.all([
        apiClient.listProjects().catch(() => []),
        apiClient.listAgents().catch(() => []),
        apiClient.listSkills().catch(() => []),
        apiClient.listRuntimeAdapters().catch(() => []),
        apiClient.listUsageOverview().catch(() => [])
      ]);
      setProjects(projs as Project[]);
      setAgents(ags as Agent[]);
      setSkills(sks as Skill[]);
      setRuntimes(rts as AgentRuntime[]);
      setUsage(usg as UsageOverviewItem[]);
    } catch (err) {
      console.error("Error fetching dashboard data", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  // Calcular totales de consumo
  const totalSpend = usage.reduce((sum, item) => sum + Number(item.totalCostUSD || 0), 0);
  const totalTokens = usage.reduce((sum, item) => sum + Number(item.totalInputTokens || 0) + Number(item.totalOutputTokens || 0), 0);

  // Renderizar estado de salud
  const renderHealthBadge = (status: string) => {
    switch (status.toLowerCase()) {
      case 'ok':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <CheckCircle size={12} /> OK
          </span>
        );
      case 'degraded':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <AlertTriangle size={12} /> DEGRADADO
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-red-500/10 text-red-400 border border-red-500/20">
            <XCircle size={12} /> DOWN
          </span>
        );
    }
  };

  if (loading) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <RefreshCw className="animate-spin text-primary" size={40} />
          <p className="text-muted-foreground text-sm">Cargando telemetría y estado de BattOS...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* 1. Resumen de contadores (Top Cards) */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
        {/* Card Proyectos */}
        <div
          onClick={() => onNavigate?.('workboard')}
          title="Abrir Work Board"
          className="glass-panel p-4 rounded-xl flex items-center justify-between transition-all hover:translate-y-[-2px] cursor-pointer"
        >
          <div>
            <p className="text-muted-foreground text-xs font-medium uppercase tracking-wider">Proyectos</p>
            <h3 className="text-2xl font-bold mt-1 text-white">{projects.length}</h3>
            <p className="text-[10px] text-emerald-400 mt-1">● Activos</p>
          </div>
          <div className="p-3 rounded-lg bg-blue-500/10 text-blue-400">
            <Folder size={20} />
          </div>
        </div>

        {/* Card Agentes */}
        <div
          onClick={() => onNavigate?.('agents')}
          title="Abrir Agents Registry"
          className="glass-panel p-4 rounded-xl flex items-center justify-between transition-all hover:translate-y-[-2px] cursor-pointer"
        >
          <div>
            <p className="text-muted-foreground text-xs font-medium uppercase tracking-wider">Agentes</p>
            <h3 className="text-2xl font-bold mt-1 text-white">{agents.length}</h3>
            <p className="text-[10px] text-indigo-400 mt-1">● {agents.filter(a => a.status === 'active').length} Online</p>
          </div>
          <div className="p-3 rounded-lg bg-indigo-500/10 text-indigo-400">
            <Cpu size={20} />
          </div>
        </div>

        {/* Card Habilidades */}
        <div
          onClick={() => onNavigate?.('agents')}
          title="Abrir Agents Registry (skills)"
          className="glass-panel p-4 rounded-xl flex items-center justify-between transition-all hover:translate-y-[-2px] cursor-pointer"
        >
          <div>
            <p className="text-muted-foreground text-xs font-medium uppercase tracking-wider">Skills</p>
            <h3 className="text-2xl font-bold mt-1 text-white">{skills.length}</h3>
            <p className="text-[10px] text-amber-400 mt-1">● Registradas</p>
          </div>
          <div className="p-3 rounded-lg bg-amber-500/10 text-amber-400">
            <Award size={20} />
          </div>
        </div>

        {/* Card Runtimes */}
        <div
          onClick={() => onNavigate?.('controlroom')}
          title="Abrir Control Room (runtimes y CLIs)"
          className="glass-panel p-4 rounded-xl flex items-center justify-between transition-all hover:translate-y-[-2px] cursor-pointer"
        >
          <div>
            <p className="text-muted-foreground text-xs font-medium uppercase tracking-wider">Runtimes</p>
            <h3 className="text-2xl font-bold mt-1 text-white">{runtimes.length}</h3>
            <p className="text-[10px] text-emerald-400 mt-1">● {runtimes.filter(r => r.status === 'configured' || r.status === 'detected').length} Configurados</p>
          </div>
          <div className="p-3 rounded-lg bg-emerald-500/10 text-emerald-400">
            <Zap size={20} />
          </div>
        </div>

        {/* Card Salud General */}
        <div
          onClick={() => document.getElementById('subsystems-panel')?.scrollIntoView({ behavior: 'smooth' })}
          title="Ver estado de subsistemas"
          className="glass-panel p-4 rounded-xl flex items-center justify-between transition-all hover:translate-y-[-2px] cursor-pointer"
        >
          <div>
            <p className="text-muted-foreground text-xs font-medium uppercase tracking-wider">Salud OS</p>
            <h3 className="text-md font-bold mt-2 text-white">
              {metrics ? renderHealthBadge(metrics.overall) : (
                <span className="text-muted-foreground">Desconectado</span>
              )}
            </h3>
            <p className="text-[10px] text-muted-foreground mt-1">Todos los sistemas</p>
          </div>
          <div className="p-3 rounded-lg bg-rose-500/10 text-rose-400">
            <Shield size={20} />
          </div>
        </div>
      </div>

      {/* 2. Sección de Telemetría en Vivo y Consumo */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Telemetría (CPU, Memoria, Red) */}
        <div className="glass-panel p-5 rounded-xl lg:col-span-2 space-y-6">
          <div className="flex items-center justify-between">
            <h3 className="text-base font-bold text-white flex items-center gap-2">
              <Activity size={18} className="text-primary" />
              Telemetría de Sistema en Tiempo Real
            </h3>
            <span className="text-[10px] text-muted-foreground bg-muted px-2 py-1 rounded">Stream SSE</span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6">
            {/* CPU */}
            <div className="space-y-2">
              <div className="flex justify-between text-xs">
                <span className="text-muted-foreground">Uso de CPU</span>
                <span className="font-semibold text-white">{metrics?.metrics.cpuPercent.toFixed(1) || '0.0'}%</span>
              </div>
              <div className="h-2 w-full bg-gray-800 rounded-full overflow-hidden">
                <div 
                  className="h-full bg-emerald-500 transition-all duration-500" 
                  style={{ width: `${metrics?.metrics.cpuPercent || 0}%` }}
                />
              </div>
            </div>

            {/* Memoria */}
            <div className="space-y-2">
              <div className="flex justify-between text-xs">
                <span className="text-muted-foreground">Memoria RAM</span>
                <span className="font-semibold text-white">{metrics?.metrics.memPercent.toFixed(1) || '0.0'}%</span>
              </div>
              <div className="h-2 w-full bg-gray-800 rounded-full overflow-hidden">
                <div 
                  className="h-full bg-indigo-500 transition-all duration-500" 
                  style={{ width: `${metrics?.metrics.memPercent || 0}%` }}
                />
              </div>
              <div className="text-[10px] text-muted-foreground text-right">
                {metrics?.metrics.memUsedMB || 0} / {metrics?.metrics.memTotalMB || 0} MB
              </div>
            </div>

            {/* Disco */}
            <div className="space-y-2">
              <div className="flex justify-between text-xs">
                <span className="text-muted-foreground flex items-center gap-1">
                  <HardDrive size={11} /> Disco
                </span>
                <span className="font-semibold text-white">{(metrics?.metrics.diskPercent || 0).toFixed(1)}%</span>
              </div>
              <div className="h-2 w-full bg-gray-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-amber-500 transition-all duration-500"
                  style={{ width: `${metrics?.metrics.diskPercent || 0}%` }}
                />
              </div>
              <div className="text-[10px] text-muted-foreground text-right">
                {(metrics?.metrics.diskUsedGB || 0).toFixed(1)} / {(metrics?.metrics.diskTotalGB || 0).toFixed(1)} GB
              </div>
            </div>

            {/* Red */}
            <div className="space-y-2">
              <div className="flex justify-between text-xs">
                <span className="text-muted-foreground">Tráfico de Red</span>
                <span className="font-semibold text-white">Activo</span>
              </div>
              <div className="text-xs space-y-1 text-muted-foreground">
                <div className="flex justify-between">
                  <span>Subida:</span>
                  <span className="text-white font-medium">{(metrics?.metrics.netUploadKBps || 0).toFixed(1)} KB/s</span>
                </div>
                <div className="flex justify-between">
                  <span>Bajada:</span>
                  <span className="text-white font-medium">{(metrics?.metrics.netDownloadKBps || 0).toFixed(1)} KB/s</span>
                </div>
              </div>
            </div>
          </div>

          {/* Gráfico SVG de simulación de carga histórico */}
          <div className="pt-2">
            <div className="h-24 w-full flex items-end gap-1">
              {Array.from({ length: 40 }).map((_, i) => {
                const height = metrics ? Math.max(10, Math.min(90, (metrics.metrics.cpuPercent * 1.5) + (Math.sin(i + (metrics.metrics.cpuPercent * 0.1)) * 15))) : 10;
                return (
                  <div 
                    key={i} 
                    className="flex-1 bg-emerald-500/20 rounded-t hover:bg-emerald-400/50 transition-all cursor-pointer"
                    style={{ height: `${height}%` }}
                    title={`Punto de muestreo ${i}`}
                  />
                );
              })}
            </div>
            <div className="flex justify-between text-[10px] text-muted-foreground pt-1">
              <span>Hace 1 min</span>
              <span>Tiempo real</span>
            </div>
          </div>

          {/* Top procesos por memoria */}
          <div className="pt-4 border-t border-gray-800/80 space-y-2">
            <div className="flex items-center justify-between">
              <h4 className="text-xs font-bold text-white flex items-center gap-1.5">
                <Activity size={13} className="text-primary" /> Top Procesos (por memoria)
              </h4>
              <span className="text-[10px] text-muted-foreground">
                {(metrics?.metrics.topProcesses || []).length} procesos
              </span>
            </div>

            {(metrics?.metrics.topProcesses || []).length === 0 ? (
              <p className="text-[10px] text-muted-foreground italic text-center py-2">
                Sin datos de procesos en el stream de telemetría.
              </p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-[10px]">
                  <thead>
                    <tr className="border-b border-gray-800 text-muted-foreground uppercase">
                      <th className="py-1.5 font-semibold">PID</th>
                      <th className="py-1.5 font-semibold">Proceso</th>
                      <th className="py-1.5 font-semibold text-right">CPU %</th>
                      <th className="py-1.5 font-semibold text-right">Mem MB</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-800/60 text-gray-200">
                    {(metrics?.metrics.topProcesses || []).map(proc => (
                      <tr key={proc.pid} className="hover:bg-gray-900/30">
                        <td className="py-1.5 font-mono text-muted-foreground">{proc.pid}</td>
                        <td className="py-1.5 font-medium text-white truncate max-w-[180px]">{proc.name}</td>
                        <td className="py-1.5 text-right font-mono text-emerald-400">{proc.cpuPercent.toFixed(1)}</td>
                        <td className="py-1.5 text-right font-mono text-indigo-300">{proc.memMB.toFixed(0)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>

        {/* Consumo de LLM (Usage) */}
        <div className="glass-panel p-5 rounded-xl space-y-6">
          <h3 className="text-base font-bold text-white flex items-center gap-2">
            <DollarSign size={18} className="text-primary" />
            Consumo Acumulado (Usage)
          </h3>

          <div className="space-y-4">
            {/* Gasto Total */}
            <div className="p-3 rounded-lg bg-gray-900/50 border border-gray-800">
              <div className="flex justify-between items-center">
                <span className="text-xs text-muted-foreground">Gasto mensual total</span>
                <span className="text-lg font-bold text-white">${totalSpend.toFixed(4)} USD</span>
              </div>
              <div className="mt-2 text-[10px] text-muted-foreground flex justify-between">
                <span>Límite (Configuración): $10.00</span>
                <span>{((totalSpend / 10.0) * 100).toFixed(1)}% usado</span>
              </div>
              <div className="h-1.5 w-full bg-gray-800 rounded-full overflow-hidden mt-1">
                <div 
                  className="h-full bg-primary" 
                  style={{ width: `${Math.min(100, (totalSpend / 10.0) * 100)}%` }}
                />
              </div>
            </div>

            {/* Tokens Utilizados */}
            <div className="p-3 rounded-lg bg-gray-900/50 border border-gray-800">
              <div className="flex justify-between items-center">
                <span className="text-xs text-muted-foreground">Tokens consumidos</span>
                <span className="text-md font-bold text-white">{(totalTokens / 1000).toFixed(1)}k</span>
              </div>
              <div className="mt-2 text-[10px] text-muted-foreground flex justify-between">
                <span>Límite (Configuración): 200M</span>
                <span>{((totalTokens / 200000000) * 100).toFixed(4)}% usado</span>
              </div>
              <div className="h-1.5 w-full bg-gray-800 rounded-full overflow-hidden mt-1">
                <div 
                  className="h-full bg-indigo-500" 
                  style={{ width: `${Math.min(100, (totalTokens / 200000000) * 100)}%` }}
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* 3. Tabla de Distribución de Uso e Historial de Eventos */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Distribución por Proyecto */}
        <div className="glass-panel p-5 rounded-xl lg:col-span-2 space-y-4">
          <h3 className="text-base font-bold text-white flex items-center gap-2">
            <BarChart2 size={18} className="text-primary" />
            Distribución de Consumo por Proyecto / Agente
          </h3>

          {usage.length === 0 ? (
            <div className="flex h-36 items-center justify-center text-muted-foreground text-sm">
              No hay eventos de consumo registrados en el sistema.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b border-gray-800 text-muted-foreground">
                    <th className="py-2.5">Proyecto</th>
                    <th className="py-2.5">Agente</th>
                    <th className="py-2.5">Proveedor/Modelo</th>
                    <th className="py-2.5 text-right">Peticiones</th>
                    <th className="py-2.5 text-right">Tokens</th>
                    <th className="py-2.5 text-right">Costo USD</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-800 text-gray-200">
                  {usage.map((item, idx) => (
                    <tr key={idx} className="hover:bg-gray-900/30">
                      <td className="py-3 font-semibold text-white">{item.projectId || 'Global'}</td>
                      <td className="py-3">
                        <span className="px-2 py-0.5 rounded bg-indigo-500/10 text-indigo-300">
                          {item.agentId}
                        </span>
                      </td>
                      <td className="py-3 text-muted-foreground">
                        {item.providerId} / <span className="text-gray-300">{item.modelId}</span>
                      </td>
                      <td className="py-3 text-right">{item.totalRequests}</td>
                      <td className="py-3 text-right">
                        {((item.totalInputTokens + item.totalOutputTokens) / 1000).toFixed(1)}k
                      </td>
                      <td className="py-3 text-right font-mono text-emerald-400">
                        ${Number(item.totalCostUSD).toFixed(5)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Salud de Subsistemas */}
        <div id="subsystems-panel" className="glass-panel p-5 rounded-xl space-y-4">
          <h3 className="text-base font-bold text-white flex items-center gap-2">
            <Shield size={18} className="text-primary" />
            Estado de Subsistemas
          </h3>

          <div className="space-y-3">
            {metrics?.subsystems.map((sub, idx) => (
              <div key={idx} className="flex justify-between items-center p-2.5 rounded bg-gray-900/40 border border-gray-800/80">
                <div>
                  <h4 className="text-xs font-semibold text-white capitalize">{sub.name}</h4>
                  <p className="text-[10px] text-muted-foreground">{sub.detail}</p>
                </div>
                <div className="text-right">
                  {renderHealthBadge(sub.status)}
                  {sub.latencyMs !== undefined && (
                    <p className="text-[9px] text-muted-foreground mt-0.5">{sub.latencyMs} ms</p>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
