import { useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  Circle,
  CheckCircle2,
  XCircle,
  Loader2,
  Server,
  ArrowLeft,
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useServerProvisioning } from '../../hooks/useServerProvisioning';
import type { ProvisioningStep } from '../../types/server';

interface ProvisioningProgressProps {
  serverId: string;
  className?: string;
}

function formatElapsed(startedAt: string): string {
  const start = new Date(startedAt).getTime();
  const elapsed = Math.max(0, Date.now() - start);
  const minutes = Math.floor(elapsed / 60000);
  const seconds = Math.floor((elapsed % 60000) / 1000);
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

function StepIcon({ status }: { status: string }) {
  switch (status) {
    case 'COMPLETED':
      return <CheckCircle2 className="h-5 w-5 text-green-400" />;
    case 'IN_PROGRESS':
      return <Loader2 className="h-5 w-5 animate-spin text-blue-400" />;
    case 'FAILED':
      return <XCircle className="h-5 w-5 text-red-400" />;
    default:
      return <Circle className="h-5 w-5 text-slate-500" />;
  }
}

function StepList({ steps }: { steps: ProvisioningStep[] }) {
  return (
    <div className="space-y-2">
      {steps.map((step) => (
        <div
          key={step.name}
          className={cn(
            'flex items-center gap-3 rounded-lg px-4 py-3 transition-colors duration-300',
            step.status === 'IN_PROGRESS' && 'bg-slate-700/50',
            step.status === 'COMPLETED' && 'bg-slate-800',
            step.status === 'FAILED' && 'bg-red-900/20',
            step.status === 'PENDING' && 'bg-slate-800'
          )}
        >
          <StepIcon status={step.status} />
          <span
            className={cn(
              'flex-1 text-sm',
              step.status === 'PENDING' && 'text-slate-500',
              step.status === 'IN_PROGRESS' && 'text-blue-300',
              step.status === 'COMPLETED' && 'text-slate-300',
              step.status === 'FAILED' && 'text-red-300'
            )}
          >
            {step.message}
          </span>
          <span className="text-xs text-slate-500">
            {step.status === 'COMPLETED' && 'Done'}
            {step.status === 'IN_PROGRESS' && 'In Progress'}
            {step.status === 'FAILED' && 'Failed'}
          </span>
        </div>
      ))}
    </div>
  );
}

const operationLabels: Record<string, string> = {
  CREATE: 'Creating',
  UPDATE: 'Updating',
  DESTROY: 'Destroying',
};

export type { ProvisioningProgressProps };

export function ProvisioningProgress({
  serverId,
  className,
}: ProvisioningProgressProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { data, isLoading, isError, error, refetch } =
    useServerProvisioning(serverId);
  const [elapsed, setElapsed] = useState('0:00');

  const serverName =
    (location.state as { serverName?: string })?.serverName ?? serverId;

  const startedAt = data?.startedAt;
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (!startedAt) {
      if (timerRef.current) clearInterval(timerRef.current);
      return;
    }

    timerRef.current = setInterval(() => {
      setElapsed(formatElapsed(startedAt));
    }, 1000);

    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [startedAt]);

  if (isLoading) {
    return (
      <div className={cn('max-w-2xl mx-auto py-8', className)}>
        <div className="animate-pulse space-y-6">
          <div className="h-8 w-64 bg-slate-700 rounded" />
          <div className="h-64 bg-slate-700 rounded-lg" />
        </div>
      </div>
    );
  }

  if (isError) {
    const is404 = error?.message === 'No provisioning in progress';

    if (is404) {
      return (
        <div className={cn('max-w-2xl mx-auto py-8', className)}>
          <div className="rounded-lg bg-slate-800 p-8 text-center">
            <Server className="mx-auto h-12 w-12 text-slate-500" />
            <h2 className="mt-4 text-lg font-semibold text-white">
              No provisioning in progress
            </h2>
            <p className="mt-2 text-sm text-slate-400">
              This server is not currently being provisioned.
            </p>
            <button
              onClick={() => navigate('/')}
              className="mt-6 inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 transition-colors"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to Dashboard
            </button>
          </div>
        </div>
      );
    }

    return (
      <div className={cn('max-w-2xl mx-auto py-8', className)}>
        <div className="rounded-lg bg-slate-800 p-8 text-center">
          <XCircle className="mx-auto h-12 w-12 text-red-400" />
          <h2 className="mt-4 text-lg font-semibold text-white">
            Failed to load status
          </h2>
          <p className="mt-2 text-sm text-red-300">{error?.message}</p>
          <button
            onClick={() => refetch()}
            className="mt-6 inline-flex items-center gap-2 rounded-lg bg-slate-700 px-4 py-2 text-sm font-medium text-white hover:bg-slate-600 transition-colors"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  if (!data) return null;

  const operationLabel = operationLabels[data.operation] ?? 'Operating on';
  const isComplete = data.state === 'COMPLETED';
  const isFailed = data.state === 'FAILED';

  return (
    <div className={cn('max-w-2xl mx-auto py-8', className)}>
      <div className="rounded-lg bg-slate-800 border border-slate-700 overflow-hidden">
        {isComplete && (
          <div className="bg-green-900/30 px-6 py-4 border-b border-slate-700">
            <div className="flex items-center gap-3">
              <CheckCircle2 className="h-6 w-6 text-green-400" />
              <div>
                <h2 className="text-lg font-semibold text-white">Completed</h2>
                <p className="text-sm text-green-300">
                  {operationLabel} server{' '}
                  <span className="font-medium">{serverName}</span> finished
                  successfully.
                </p>
              </div>
            </div>
          </div>
        )}

        {isFailed && (
          <div className="bg-red-900/30 px-6 py-4 border-b border-slate-700">
            <div className="flex items-center gap-3">
              <XCircle className="h-6 w-6 text-red-400" />
              <div>
                <h2 className="text-lg font-semibold text-white">Failed</h2>
                <p className="text-sm text-red-300">
                  {operationLabel} server{' '}
                  <span className="font-medium">{serverName}</span> encountered
                  an error.
                </p>
              </div>
            </div>
          </div>
        )}

        <div className="px-6 py-5 space-y-6">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-white">
              {operationLabel} Server:{' '}
              <span className="text-blue-300">{serverName}</span>
            </h2>
            <div className="flex items-center gap-2 text-sm text-slate-400">
              <span>{elapsed}</span>
            </div>
          </div>

          <StepList steps={data.steps} />

          <div>
            <div className="flex items-center justify-between text-sm mb-2">
              <span className="text-slate-400">Progress</span>
              <span className="text-slate-300 font-medium">
                {data.progress}%
              </span>
            </div>
            <div className="h-2 rounded-full bg-slate-700 overflow-hidden">
              <div
                className={cn(
                  'h-full rounded-full transition-all duration-500 ease-in-out',
                  isComplete && 'bg-green-500',
                  isFailed && 'bg-red-500',
                  !isComplete && !isFailed && 'bg-blue-500'
                )}
                style={{ width: `${data.progress}%` }}
              />
            </div>
          </div>

          {isFailed && data.error && (
            <div className="rounded-lg bg-red-900/20 border border-red-800/50 px-4 py-3">
              <p className="text-sm text-red-300">
                <span className="font-semibold">Error:</span> {data.error}
              </p>
            </div>
          )}
        </div>

        <div className="bg-slate-900/50 px-6 py-4 border-t border-slate-700 flex justify-end gap-3">
          <button
            onClick={() => navigate('/')}
            className="inline-flex items-center gap-2 rounded-lg bg-slate-700 px-4 py-2 text-sm font-medium text-white hover:bg-slate-600 transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            {isComplete || isFailed ? 'Back to Dashboard' : 'Dashboard'}
          </button>
          {isFailed && (
            <button
              onClick={() => refetch()}
              className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 transition-colors"
            >
              <Loader2 className="h-4 w-4" />
              Retry
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
