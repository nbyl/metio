import { useMutation, useQueryClient } from '@tanstack/react-query';

async function initializeSetup(): Promise<void> {
  const response = await fetch('/api/setup/initialize', { method: 'POST' });

  if (response.status === 401) {
    window.location.href = '/auth/login';
    throw new Error('Session expired');
  }

  if (!response.ok) {
    throw new Error('Initialization failed');
  }
}

export function useInitialize() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: initializeSetup,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['setup', 'status'] });
    },
  });
}
