/**
 * Represents the standardized success response from the API.
 */
export interface ApiResponse<T> {
  success: boolean;
  code: number;
  message: string;
  data: T;
}

/**
 * Download type for a task.
 */
export enum DownloadType {
  M3U8 = 'm3u8',
  Bilibili = 'bilibili',
  Direct = 'direct',
}

/**
 * Status of a download task.
 */
export enum TaskStatus {
  Pending = 'pending',
  Downloading = 'downloading',
  Success = 'success',
  Failed = 'failed',
  Stopped = 'stopped',
}

/**
 * Represents a single download task's information.
 */
export interface Task {
  id: string;
  type: DownloadType;
  url: string;
  name: string;
  status: TaskStatus;
  percent?: number;
  speed?: string;
  isLive?: boolean;
  error?: string;
}

/**
 * Parameters for creating a new download task.
 */
export interface CreateTaskParams {
  id?: string;
  type: DownloadType;
  url: string;
  name: string;
  folder?: string;
  headers?: string[];
}

/**
 * Response after creating a task.
 */
export interface CreateTaskResponse {
  id: string;
  message: string;
  status: string;
}

/**
 * Response for a list of tasks.
 */
export interface TaskListResponse {
  tasks: Task[];
  total: number;
}

/**
 * Parameters for updating the server configuration.
 * All fields are optional.
 */
export interface UpdateConfigParams {
  maxRunner?: number;
  localDir?: string;
  deleteSegments?: boolean;
  proxy?: string;
  useProxy?: boolean;
}

/**
 * Payload for SSE events that indicate a change in task status.
 */
export interface TaskEventPayload {
  id: string;
}

/**
 * Payload for the 'download-failed' SSE event.
 */
export interface TaskFailedEventPayload {
  id: string;
  error: string;
}

// #region Event Emitter Types

/**
 * Maps the SSE event names to their respective payload types.
 */
export interface TaskEventMap {
  'download-start': TaskEventPayload;
  'download-success': TaskEventPayload;
  'download-failed': TaskFailedEventPayload;
  'download-stop': TaskEventPayload;
  'open': Event;
  'error': Event;
}

/**
 * Describes a generic, strongly-typed event emitter.
 */
export interface TypedEventEmitter<TEventMap extends Record<string, any>> {
  on<TEventName extends keyof TEventMap>(
    eventName: TEventName,
    listener: (payload: TEventMap[TEventName]) => void
  ): this;

  off<TEventName extends keyof TEventMap>(
    eventName: TEventName,
    listener: (payload: TEventMap[TEventName]) => void
  ): this;

  once<TEventName extends keyof TEventMap>(
    eventName: TEventName,
    listener: (payload: TEventMap[TEventName]) => void
  ): this;

  emit<TEventName extends keyof TEventMap>(
    eventName: TEventName,
    payload: TEventMap[TEventName]
  ): boolean;

  removeAllListeners<TEventName extends keyof TEventMap>(
    eventName?: TEventName
  ): this;

  /**
   * Closes the underlying connection or resource.
   */
  close(): void;
}

/**
 * A strongly-typed event emitter for task-related SSE events.
 */
export type TaskEventEmitter = TypedEventEmitter<TaskEventMap>;

// #endregion
