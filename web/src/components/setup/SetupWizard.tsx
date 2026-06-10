import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { CheckCircle, AlertCircle, Loader2, Server, ArrowRight } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '../ui/Card';
import { Button } from '../ui/Button';
import { cn } from '../../lib/utils';
import { useSetupStatus } from '../../hooks/useSetupStatus';
import { useInitialize } from '../../hooks/useInitialize';
import type { SetupStatus } from '../../types/setup';

interface StepProps {
  status: SetupStatus | undefined;
  isLoading: boolean;
}

function WelcomeStep({ status, isLoading }: StepProps) {
  return (
    <CardContent>
      <div className="flex flex-col items-center justify-center py-8 text-center">
        <Server className="h-16 w-16 text-green-500 mb-4" />
        <h3 className="text-xl font-semibold text-white mb-2">
          Welcome to Metio
        </h3>
        <p className="text-slate-400 max-w-md mb-2">
          Metio manages Minecraft servers on Google Cloud. Before you can create
          servers, we need to verify that your GCP project is properly configured.
        </p>
        {isLoading ? (
          <div className="flex items-center gap-2 mt-4 text-slate-400">
            <Loader2 className="h-4 w-4 animate-spin" />
            Checking setup status...
          </div>
        ) : status?.initialized ? (
          <div className="flex items-center gap-2 mt-4 text-green-400">
            <CheckCircle className="h-5 w-5" />
            Already initialized
          </div>
        ) : null}
      </div>
    </CardContent>
  );
}

function ValidationStep({ status, isLoading }: StepProps) {
  const checks = status?.checks;
  const checksLoading = isLoading || !checks;

  const apiCount = checks ? Object.keys(checks.apis).length : 0;
  const enabledApis = checks
    ? Object.values(checks.apis).filter((a) => a.enabled).length
    : 0;
  const permCount = checks ? Object.keys(checks.permissions).length : 0;
  const grantedPerms = checks
    ? Object.values(checks.permissions).filter((p) => p.granted).length
    : 0;

  return (
    <CardContent>
      {checksLoading ? (
        <div className="flex items-center justify-center gap-2 py-8 text-slate-400">
          <Loader2 className="h-5 w-5 animate-spin" />
          Running validation...
        </div>
      ) : (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <span className="text-white">GCP APIs</span>
            <span className={enabledApis === apiCount ? 'text-green-400' : 'text-yellow-400'}>
              {enabledApis}/{apiCount} enabled
            </span>
          </div>
          <div className="space-y-2">
            {Object.entries(checks.apis).map(([api, result]) => (
              <div key={api} className="flex items-center justify-between text-sm">
                <span className="text-slate-300">{api}</span>
                {result.enabled ? (
                  <CheckCircle className="h-4 w-4 text-green-400" />
                ) : (
                  <AlertCircle className="h-4 w-4 text-red-400" />
                )}
              </div>
            ))}
          </div>

          <div className="border-t border-slate-700 pt-4">
            <div className="flex items-center justify-between">
              <span className="text-white">IAM Permissions</span>
              <span className={grantedPerms === permCount ? 'text-green-400' : 'text-yellow-400'}>
                {grantedPerms}/{permCount} granted
              </span>
            </div>
            <div className="space-y-2 mt-2">
              {Object.entries(checks.permissions).map(([perm, result]) => (
                <div key={perm} className="flex items-center justify-between text-sm">
                  <span className="text-slate-300">{perm}</span>
                  {result.granted ? (
                    <CheckCircle className="h-4 w-4 text-green-400" />
                  ) : (
                    <AlertCircle className="h-4 w-4 text-red-400" />
                  )}
                </div>
              ))}
            </div>
          </div>

          {checks.fixes.length > 0 && (
            <div className="border-t border-slate-700 pt-4">
              <p className="text-yellow-400 text-sm mb-2">Required fixes:</p>
              <div className="space-y-2">
                {checks.fixes.map((fix, i) => (
                  <div key={i} className="flex items-start gap-2 text-sm">
                    <AlertCircle className="h-4 w-4 text-yellow-400 mt-0.5 shrink-0" />
                    <span className="text-slate-300">
                      {fix.type === 'enable_api' && `Enable API: ${fix.api}`}
                      {fix.type === 'grant_role' && `Grant role ${fix.role} (${fix.permission})`}
                    </span>
                  </div>
                ))}
              </div>
              <p className="text-xs text-slate-500 mt-2">
                Open the GCP console links above to fix these issues, then refresh.
              </p>
            </div>
          )}

          {checks.valid && (
            <div className="flex items-center justify-center gap-2 text-green-400 pt-2">
              <CheckCircle className="h-5 w-5" />
              All checks passed
            </div>
          )}
        </div>
      )}
    </CardContent>
  );
}

function InitializeStep() {
  const initializeMutation = useInitialize();
  const [started, setStarted] = useState(false);

  const handleInitialize = () => {
    setStarted(true);
    initializeMutation.mutate();
  };

  return (
    <CardContent>
      <div className="flex flex-col items-center text-center py-4">
        <Server className="h-12 w-12 text-slate-500 mb-4" />
        <p className="text-slate-300 mb-6 max-w-md">
          We'll now create the Pulumi state bucket in your GCP project. This
          bucket stores the infrastructure state for your servers.
        </p>
        {!started ? (
          <Button variant="primary" onClick={handleInitialize}>
            Create State Bucket
            <ArrowRight className="h-4 w-4" />
          </Button>
        ) : initializeMutation.isPending ? (
          <div className="flex items-center gap-2 text-slate-400">
            <Loader2 className="h-5 w-5 animate-spin" />
            Creating state bucket...
          </div>
        ) : initializeMutation.isError ? (
          <div className="flex flex-col items-center gap-2">
            <AlertCircle className="h-8 w-8 text-red-400" />
            <p className="text-red-400 text-sm">{initializeMutation.error.message}</p>
            <Button variant="outline" onClick={handleInitialize}>
              Retry
            </Button>
          </div>
        ) : (
          <div className="flex items-center gap-2 text-green-400">
            <CheckCircle className="h-6 w-6" />
            State bucket ready
          </div>
        )}
      </div>
    </CardContent>
  );
}

function CompleteStep() {
  const navigate = useNavigate();

  return (
    <CardContent>
      <div className="flex flex-col items-center justify-center py-8 text-center">
        <CheckCircle className="h-16 w-16 text-green-500 mb-4" />
        <h3 className="text-xl font-semibold text-white mb-2">
          Setup Complete
        </h3>
        <p className="text-slate-400 max-w-md mb-6">
          Your GCP project is now configured. You can start creating and managing
          Minecraft servers.
        </p>
        <Button variant="primary" onClick={() => navigate('/')}>
          Go to Dashboard
          <ArrowRight className="h-4 w-4" />
        </Button>
      </div>
    </CardContent>
  );
}

const STEPS = ['Welcome', 'Validate', 'Initialize', 'Complete'] as const;

export function SetupWizard() {
  const { data: status, isLoading } = useSetupStatus();
  const [step, setStep] = useState(0);
  const initializeMutation = useInitialize();

  const canAdvance = () => {
    if (step === 0) return true;
    if (step === 1) return status?.checks?.valid ?? false;
    if (step === 2) return initializeMutation.isSuccess;
    return true;
  };

  const handleNext = () => {
    if (step < STEPS.length - 1) {
      setStep(step + 1);
    }
  };

  const stepIcons = [
    <Server key="welcome" className="h-4 w-4" />,
    <AlertCircle key="validate" className="h-4 w-4" />,
    <Loader2 key="init" className="h-4 w-4" />,
    <CheckCircle key="complete" className="h-4 w-4" />,
  ];

  return (
    <div className="max-w-2xl mx-auto">
      <Card>
        <CardHeader>
          <CardTitle>
            <span>Setup</span>
            <span className="text-sm text-slate-400 font-normal">
              Step {step + 1} of {STEPS.length}
            </span>
          </CardTitle>
        </CardHeader>

        <div className="px-6">
          <div className="flex items-center justify-between mb-6">
            {STEPS.map((label, i) => (
              <div key={label} className="flex items-center gap-2">
                <div
                  className={cn(
                    'flex items-center justify-center w-8 h-8 rounded-full text-xs font-medium transition-colors',
                    i === step
                      ? 'bg-green-600 text-white'
                      : i < step
                        ? 'bg-green-900 text-green-300'
                        : 'bg-slate-700 text-slate-400'
                  )}
                >
                  {stepIcons[i]}
                </div>
                <span
                  className={cn(
                    'text-sm hidden sm:inline',
                    i === step ? 'text-white' : 'text-slate-500'
                  )}
                >
                  {label}
                </span>
              </div>
            ))}
          </div>
        </div>

        {step === 0 && <WelcomeStep status={status} isLoading={isLoading} />}
        {step === 1 && <ValidationStep status={status} isLoading={isLoading} />}
        {step === 2 && <InitializeStep />}
        {step === 3 && <CompleteStep />}

        <div className="px-6 pb-6">
          <div className="flex justify-between">
            {step > 0 ? (
              <Button
                variant="outline"
                onClick={() => setStep(step - 1)}
              >
                Back
              </Button>
            ) : (
              <div />
            )}
            {step < STEPS.length - 1 && (
              <Button
                variant="primary"
                onClick={handleNext}
                disabled={!canAdvance()}
              >
                {step === STEPS.length - 2 ? 'Finish' : 'Next'}
                <ArrowRight className="h-4 w-4" />
              </Button>
            )}
          </div>
        </div>
      </Card>
    </div>
  );
}
