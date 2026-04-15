import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type {
  WhitelistResponse,
  WhitelistPlayer,
  AddPlayerRequest,
} from '../types/whitelist';
import type { APIError } from '../types/server';

/**
 * Fetches the whitelist from the API
 */
async function fetchWhitelist(): Promise<WhitelistResponse> {
  const response = await fetch('/api/server/whitelist', {
    credentials: 'include',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to fetch whitelist');
  }
  return response.json();
}

/**
 * Adds a player to the whitelist
 */
async function addPlayer(username: string): Promise<WhitelistPlayer> {
  const response = await fetch('/api/server/whitelist', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username } as AddPlayerRequest),
    credentials: 'include',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to add player');
  }
  return response.json();
}

/**
 * Removes a player from the whitelist
 */
async function removePlayer(uuid: string): Promise<void> {
  const response = await fetch(`/api/server/whitelist/${uuid}`, {
    method: 'DELETE',
    credentials: 'include',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to remove player');
  }
}

/**
 * Sets whitelist enabled status
 */
async function setWhitelistEnabled(enabled: boolean): Promise<void> {
  const response = await fetch('/api/server/whitelist/enabled', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
    credentials: 'include',
  });
  if (!response.ok) {
    const error: APIError = await response.json();
    throw new Error(error.error || 'Failed to update whitelist status');
  }
}

/**
 * Hook to fetch and poll whitelist data
 */
export function useWhitelist() {
  return useQuery({
    queryKey: ['whitelist'],
    queryFn: fetchWhitelist,
    refetchInterval: 10000, // Poll every 10 seconds
    refetchIntervalInBackground: false,
  });
}

/**
 * Hook to add a player to the whitelist
 */
export function useAddPlayer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: addPlayer,
    onSuccess: (newPlayer) => {
      // Optimistically update the cache
      queryClient.setQueryData<WhitelistResponse>(['whitelist'], (old) => {
        if (!old) return old;
        return {
          ...old,
          players: [...old.players, newPlayer],
        };
      });
      toast.success(`Added ${newPlayer.username} to whitelist`);
      queryClient.invalidateQueries({ queryKey: ['whitelist'] });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });
}

/**
 * Hook to remove a player from the whitelist
 */
export function useRemovePlayer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: removePlayer,
    onMutate: async (uuid) => {
      // Cancel any outgoing refetches
      await queryClient.cancelQueries({ queryKey: ['whitelist'] });

      // Snapshot the previous value
      const previousWhitelist =
        queryClient.getQueryData<WhitelistResponse>(['whitelist']);

      // Optimistically update
      queryClient.setQueryData<WhitelistResponse>(['whitelist'], (old) => {
        if (!old) return old;
        return {
          ...old,
          players: old.players.filter((p) => p.uuid !== uuid),
        };
      });

      return { previousWhitelist };
    },
    onSuccess: () => {
      toast.success('Player removed from whitelist');
      queryClient.invalidateQueries({ queryKey: ['whitelist'] });
    },
    onError: (error: Error, _uuid, context) => {
      // Rollback on error
      if (context?.previousWhitelist) {
        queryClient.setQueryData(['whitelist'], context.previousWhitelist);
      }
      toast.error(error.message);
    },
  });
}

/**
 * Hook to toggle whitelist enabled status
 */
export function useToggleWhitelist() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: setWhitelistEnabled,
    onMutate: async (enabled) => {
      await queryClient.cancelQueries({ queryKey: ['whitelist'] });

      const previousWhitelist =
        queryClient.getQueryData<WhitelistResponse>(['whitelist']);

      // Optimistically update
      queryClient.setQueryData<WhitelistResponse>(['whitelist'], (old) => {
        if (!old) return old;
        return { ...old, enabled };
      });

      return { previousWhitelist };
    },
    onSuccess: (_data, enabled) => {
      toast.success(`Whitelist ${enabled ? 'enabled' : 'disabled'}`);
      queryClient.invalidateQueries({ queryKey: ['whitelist'] });
      queryClient.invalidateQueries({ queryKey: ['serverStatus'] });
    },
    onError: (error: Error, _enabled, context) => {
      if (context?.previousWhitelist) {
        queryClient.setQueryData(['whitelist'], context.previousWhitelist);
      }
      toast.error(error.message);
    },
  });
}
