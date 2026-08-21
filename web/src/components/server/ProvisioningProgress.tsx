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
import { Progress } from '../ui/Progress';
import { Button } from '../ui/Button';

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
      return <CheckCircle2 className="h-5 w-5 text-green-600" />;
    case 'IN_PROGRESS':
      return <Loader2 className="h-5 w-5 animate-spin text-blue-600" />;
    case 'FAILED':
      return <XCircle className="h-5 w-5 text-destructive" />;
    default:
      return <Circle className="h-5 w-5 text-muted-foreground" />;
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
            step.status === 'IN_PROGRESS' && 'bg-accent',
            step.status === 'COMPLETED' && 'bg-muted/50',
            step.status === 'FAILED' && 'bg-destructive/10'
          )}
        >
          <StepIcon status={step.status} />
          <span
            className={cn(
              'flex-1 text-sm',
              step.status === 'PENDING' && 'text-muted-foreground',
              step.status === 'IN_PROGRESS' && 'text-blue-700',
              step.status === 'COMPLETED' && 'text-foreground',
              step.status === 'FAILED' && 'text-destructive'
            )}
          >
            {step.message}
          </span>
          <span className="text-xs text-muted-foreground">
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
          <div className="h-8 w-64 bg-muted rounded" />
          <div className="h-64 bg-muted rounded-lg" />
        </div>
      </div>
    );
  }

  if (isError) {
    const is404 = error?.message === 'No provisioning in progress';

    if (is404) {
      return (
        <div className={cn('max-w-2xl mx-auto py-8', className)}>
          <div className="rounded-lg border bg-card p-8 text-center">
            <Server className="mx-auto h-12 w-12 text-muted-foreground" />
            <h2 className="mt-4 text-lg font-semibold text-foreground">
              No provisioning in progress
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              This server is not currently being provisioned.
            </p>
            <Button
              variant="outline"
              onClick={() => navigate('/')}
              className="mt-6"
            >
              <ArrowLeft />
              Back to Dashboard
            </Button>
          </div>
        </div>
      );
    }

    return (
      <div className={cn('max-w-2xl mx-auto py-8', className)}>
        <div className="rounded-lg border bg-card p-8 text-center">
          <XCircle className="mx-auto h-12 w-12 text-destructive" />
          <h2 className="mt-4 text-lg font-semibold text-foreground">
            Failed to load status
          </h2>
          <p className="mt-2 text-sm text-destructive">{error?.message}</p>
          <Button onClick={() => refetch()} className="mt-6">
            Retry
          </Button>
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
      <div className="rounded-lg border bg-card overflow-hidden">
        {isComplete && (
          <div className="bg-green-100 px-6 py-4 border-b border-border">
            <div className="flex items-center gap-3">
              <CheckCircle2 className="h-6 w-6 text-green-600" />
              <div>
                <h2 className="text-lg font-semibold text-foreground">
                  Completed
                </h2>
                <p className="text-sm text-green-700">
                  {operationLabel} server{' '}
                  <span className="font-medium">{serverName}</span> finished
                  successfully.
                </p>
              </div>
            </div>
          </div>
        )}

        {isFailed && (
          <div className="bg-destructive/10 px-6 py-4 border-b border-border">
            <div className="flex items-center gap-3">
              <XCircle className="h-6 w-6 text-destructive" />
              <div>
                <h2 className="text-lg font-semibold text-foreground">
                  Failed
                </h2>
                <p className="text-sm text-destructive">
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
            <h2 className="text-lg font-semibold text-foreground">
              {operationLabel} Server:{' '}
              <span className="text-blue-700">{serverName}</span>
            </h2>
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <span>{elapsed}</span>
            </div>
          </div>

          <StepList steps={data.steps} />

          <div>
            <div className="flex items-center justify-between text-sm mb-2">
              <span className="text-muted-foreground">Progress</span>
              <span className="text-foreground font-medium">
                {data.progress}%
              </span>
            </div>
            <Progress
              value={data.progress}
              aria-valuenow={data.progress}
              aria-label="Provisioning progress"
              className={cn(
                'h-2',
                isComplete && '[&>[data-slot=progress-indicator]]:bg-green-500',
                isFailed && '[&>[data-slot=progress-indicator]]:bg-red-500',
                !isComplete &&
                  !isFailed &&
                  '[&>[data-slot=progress-indicator]]:bg-blue-500'
              )}
            />
          </div>

          {isFailed && data.error && (
            <div className="rounded-lg bg-destructive/10 border border-destructive/30 px-4 py-3">
              <p className="text-sm text-destructive">
                <span className="font-semibold">Error:</span> {data.error}
              </p>
            </div>
          )}
        </div>

        <div className="bg-muted/50 px-6 py-4 border-t border-border flex justify-end gap-3">
          <Button
            variant="outline"
            onClick={() => navigate('/')}
          >
            <ArrowLeft />
            {isComplete || isFailed ? 'Back to Dashboard' : 'Dashboard'}
          </Button>
          {isFailed && (
            <Button onClick={() => refetch()}>
              <Loader2 className="animate-spin" />
              Retry
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
