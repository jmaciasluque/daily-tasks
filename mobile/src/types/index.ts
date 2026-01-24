export type Task = {
  id: number;
  title: string;
  duration: number;
  status: 'todo' | 'done';
  order: number;
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
