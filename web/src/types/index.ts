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

export type Data = {
  last_reset: string;
  next_id: number;
  tasks: Task[];
  theme_index: number;
  last_modified?: number;
};

export type ServerAction = 'loaded' | 'saved' | 'pulled' | 'pushed' | 'error' | 'in_sync';

export type ServerState = {
  action?: ServerAction;
  data: Data;
  data_path: string;
  message?: string;
  sync_configured: boolean;
  version: string;
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
