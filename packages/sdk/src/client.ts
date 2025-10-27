/* eslint-disable no-console */
import type { AxiosInstance } from 'axios';
import { createApiClient } from './api';
import { EventSource } from 'eventsource';
import type {
  ApiResponse,
  CreateTaskParams,
  CreateTaskResponse,
  Task,
  TaskEventEmitter,
  TaskListResponse,
  UpdateConfigParams,
} from './types';
import { TaskStreamEventEmitter } from './eventEmitter';

/**
 * Options for initializing the MediaGoClient.
 */
export interface MediaGoClientOptions {
  /**
   * The base URL of the MediaGo server.
   * @default 'http://localhost:8080'
   */
  baseURL?: string;
}

/**
 * The main client for interacting with the MediaGo API.
 */
export class MediaGoClient {
  public readonly api: AxiosInstance;
  private readonly baseURL: string;

  /**
   * Creates an instance of MediaGoClient.
   * @param options - Configuration options for the client.
   */
  constructor(options: MediaGoClientOptions = {}) {
    this.baseURL = options.baseURL ?? 'http://localhost:8080';
    this.api = createApiClient(this.baseURL);
  }

  /**
   * Checks the health of the server.
   * @returns A promise that resolves to the health status message.
   */
  async health(): Promise<string> {
    // The interceptor will return the text response directly
    return this.api.get('/healthy', { responseType: 'text' });
  }

  /**
   * Creates a new download task.
   * @param params - The parameters for the new task.
   * @returns The response from the server.
   */
  async createTask(
    params: CreateTaskParams,
  ): Promise<ApiResponse<CreateTaskResponse>> {
    return this.api.post('/api/tasks', params);
  }

  /**
   * Retrieves a single task by its ID.
   * @param id - The ID of the task to retrieve.
   * @returns The task information.
   */
  async getTask(id: string): Promise<ApiResponse<Task>> {
    return this.api.get(`/api/tasks/${id}`);
  }

  /**
   * Lists all current tasks.
   * @returns A list of all tasks.
   */
  async listTasks(): Promise<ApiResponse<TaskListResponse>> {
    return this.api.get('/api/tasks');
  }

  /**
   * Stops a running task.
   * @param id - The ID of the task to stop.
   * @returns A confirmation message.
   */
  async stopTask(
    id: string,
  ): Promise<ApiResponse<{ message: string }>> {
    return this.api.post(`/api/tasks/${id}/stop`);
  }

  /**
   * Updates the server configuration.
   * @param config - The configuration settings to update.
   * @returns A confirmation message.
   */
  async updateConfig(
    config: UpdateConfigParams,
  ): Promise<ApiResponse<{ message: string }>> {
    return this.api.post('/api/config', config);
  }

  /**
   * Connects to the server's real-time event stream (SSE) and returns a typed event emitter.
   *
   * @example
   * const client = new MediaGoClient();
   * const events = client.streamEvents();
   *
   * events.on('download-start', (payload) => {
   *   console.log(`Task ${payload.id} started.`);
   * });
   *
   * events.on('download-failed', (payload) => {
   *   console.error(`Task ${payload.id} failed:`, payload.error);
   * });
   *
   * events.on('error', (error) => {
   *   console.error('SSE Connection Error:', error);
   * });
   *
   * // You would need to implement a way to close the underlying EventSource.
   *
   * @returns A strongly-typed event emitter for SSE events.
   */
  streamEvents(): TaskEventEmitter {
    const eventsURL = new URL('/api/events', this.baseURL);
    const source = new EventSource(eventsURL.toString());
    return new TaskStreamEventEmitter(source);
  }
}
