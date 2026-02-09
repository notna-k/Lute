import { apiClient } from './api';
import { Machine } from '../types';

export interface CreateMachineRequest {
    name: string;
    description?: string;
    isPublic?: boolean;
    metadata?: Record<string, unknown>;
}

export interface UpdateMachineRequest {
    name?: string;
    description?: string;
    status?: string;
    isPublic?: boolean;
    metadata?: Record<string, unknown>;
}

export const machineService = {
    // Get all user's machines
    getUserMachines: async (): Promise<Machine[]> => {
        return apiClient.get<Machine[]>('/api/v1/machines');
    },

    // Get public machines
    getPublicMachines: async (): Promise<Machine[]> => {
        return apiClient.get<Machine[]>('/api/v1/machines/public');
    },

    // Get machine by ID
    getMachine: async (id: string): Promise<Machine> => {
        return apiClient.get<Machine>(`/api/v1/machines/${id}`);
    },

    // Create a new machine
    createMachine: async (data: CreateMachineRequest): Promise<Machine> => {
        return apiClient.post<Machine>('/api/v1/machines', data);
    },

    // Update a machine
    updateMachine: async (id: string, data: UpdateMachineRequest): Promise<Machine> => {
        return apiClient.put<Machine>(`/api/v1/machines/${id}`, data);
    },

    // Delete a machine
    deleteMachine: async (id: string): Promise<void> => {
        return apiClient.delete<void>(`/api/v1/machines/${id}`);
    },
};

