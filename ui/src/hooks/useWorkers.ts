import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { workerService } from '@/services/workerService';

const workerKeys = {
  all: ['workers'] as const,
  lists: () => [...workerKeys.all, 'list'] as const,
  list: (filter: string) => [...workerKeys.lists(), filter] as const,
  details: () => [...workerKeys.all, 'detail'] as const,
  detail: (id: string) => [...workerKeys.details(), id] as const,
};

/** `enabled: false` defers the fetch — used by the palette, which only needs it while open. */
export const useUserWorkers = (options?: { enabled?: boolean }) => {
  return useQuery({
    queryKey: workerKeys.list('user'),
    queryFn: workerService.getUserWorkers,
    staleTime: 30000,
    gcTime: 5 * 60 * 1000,
    enabled: options?.enabled ?? true,
  });
};

export const useWorker = (id: string) => {
  return useQuery({
    queryKey: workerKeys.detail(id),
    queryFn: () => workerService.getWorker(id),
    enabled: !!id,
    staleTime: 30000,
  });
};

export const useReEnableWorker = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => workerService.reEnableWorker(id),
    onSuccess: (data) => {
      queryClient.setQueryData(workerKeys.detail(data.id), data);
      queryClient.invalidateQueries({ queryKey: workerKeys.lists() });
    },
  });
};

export const useDeleteWorker = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => workerService.deleteWorker(id),
    onSuccess: (res, deletedId) => {
      if (res.status === 'deleted') {
        queryClient.removeQueries({ queryKey: workerKeys.detail(deletedId) });
      } else {
        queryClient.invalidateQueries({
          queryKey: workerKeys.detail(deletedId),
        });
      }
      queryClient.invalidateQueries({ queryKey: workerKeys.lists() });
    },
  });
};

const tokenKeys = ['worker-tokens'] as const;

export const useWorkerTokens = () =>
  useQuery({ queryKey: tokenKeys, queryFn: workerService.listTokens });

export const useCreateWorkerToken = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => workerService.createToken(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: tokenKeys }),
  });
};

export const useRevokeWorkerToken = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => workerService.revokeToken(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: tokenKeys }),
  });
};

export const useInstallInfo = (enabled: boolean) =>
  useQuery({
    queryKey: ['worker-install'],
    queryFn: workerService.installInfo,
    enabled,
    staleTime: Infinity,
  });
