import type { Data, ServerState, StatsSummary, Task } from '../types';

/** What the UI needs from a data source (local CLI server or hosted sync). */
export type TaskDataApi = {
  data: Data;
  serverState: ServerState | null;
  statusMsg: string;
  syncing: boolean;
  refreshing: boolean;
  refreshServerState: (successMessage?: string) => Promise<void>;
  reloadFromDisk: () => Promise<void>;
  syncFromRemote: () => Promise<void>;
  addTask: (title: string, duration: number, deadline?: string, visibility?: number[]) => void;
  editTask: (id: number, title: string, duration: number, deadline?: string, visibility?: number[]) => void;
  deleteTask: (id: number) => void;
  toggleTaskStatus: (task: Task) => void;
  skipTask: (id: number) => void;
  cycleTheme: () => void;
  chooseLocalBackend: () => Promise<void>;
  loadStats: (from: string, to: string) => Promise<StatsSummary>;
  /** Present only in the hosted build. */
  hosted: null | {
    signedIn: boolean;
    email?: string;
    signInUrl: string;
    signOut: () => void;
    deleteAccount: () => Promise<void>;
  };
};
