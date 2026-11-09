export type TaskStatus = 'todo' | 'done' | 'skipped';

export type Task = {
  id: number;
  title: string;
  duration: number;
  status: TaskStatus;
  order: number;
  deadline?: string; // HH:MM format, optional
  visibility?: number[]; // days of week (0=Sun..6=Sat); omitted = every day
};

export type TaskSnapshot = {
  id: number;
  title: string;
  duration: number;
  status: TaskStatus;
  deadline?: string;
};

export type HistoryDay = {
  date: string;
  updated_at?: number;
  tasks: TaskSnapshot[];
};

export type HistoryEvent = {
  timestamp: number;
  date: string;
  type: 'task_added' | 'task_updated' | 'task_deleted' | 'status_changed';
  task_id: number;
  title: string;
  from_status?: TaskStatus;
  to_status?: TaskStatus;
  duration?: number;
  deadline?: string;
};

export type History = {
  version: number;
  updated_at?: number;
  days: HistoryDay[];
  events: HistoryEvent[];
};

export type DailyStats = {
  date: string;
  task_count: number;
  todo_count: number;
  done_count: number;
  skipped_count: number;
  todo_duration: number;
  done_duration: number;
  skipped_duration: number;
  completion_rate: number;
};

export type TaskFrequencyStats = {
  task_id: number;
  title: string;
  recorded_days: number;
  todo_days: number;
  done_days: number;
  skipped_days: number;
  completion_rate: number;
  total_duration: number;
  done_duration: number;
  skipped_duration: number;
};

export type StatsSummary = {
  from: string;
  to: string;
  recorded_days: number;
  task_count: number;
  todo_count: number;
  done_count: number;
  skipped_count: number;
  todo_duration: number;
  done_duration: number;
  skipped_duration: number;
  completion_rate: number;
  daily: DailyStats[];
  tasks: TaskFrequencyStats[];
};

export type Data = {
  version?: number;
  last_reset: string;
  next_id: number;
  tasks: Task[];
  theme_index: number;
  last_modified?: number;
};

export type Settings = {
  baseUrl: string;
  username: string;
  password: string;
  remotePath: string;
};

export type HostedSettings = {
  apiUrl: string;
  token?: string;
  email?: string;
};

export type BackendType = 'local' | 'nextcloud' | 'hosted';

export type AppConfig = {
  backend?: BackendType;
  nextcloud?: Settings;
  hosted?: HostedSettings;
};
