export type TaskStatus = 'todo' | 'done' | 'skipped';

export type Task = {
  id: number;
  title: string;
  duration: number;
  status: TaskStatus;
  order: number;
  deadline?: string; // HH:MM format, optional
};

export type Data = {
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
