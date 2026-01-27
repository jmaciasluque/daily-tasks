import Constants from 'expo-constants';
import * as Updates from 'expo-updates';

export type AppVariant = 'production' | 'testing';

const extra = Constants.expoConfig?.extra ?? {};
const updateExtra = (Updates.manifest as { extra?: Record<string, unknown> } | undefined)?.extra ?? {};
const envAppVariant = process.env.APP_VARIANT;
const envVersionSuffix = process.env.APP_VERSION_SUFFIX;

export const appVariant: AppVariant =
  (envAppVariant as AppVariant) || (updateExtra.appVariant as AppVariant) || (extra.appVariant as AppVariant) || 'production';

export const defaultRemotePath: string =
  (updateExtra.defaultRemotePath as string) || (extra.defaultRemotePath as string) || '/remote.php/dav/files/<username>/.daily-tasks.json';

export const storagePrefix: string = (updateExtra.storagePrefix as string) || (extra.storagePrefix as string) || 'dailyTasks';

export const appVersionSuffix: string =
  envVersionSuffix || (updateExtra.appVersionSuffix as string) || (extra.appVersionSuffix as string) || '';

export const commitHash: string =
  (updateExtra.commitHash as string) || (extra.commitHash as string) || appVersionSuffix || '';

export const appVersion: string =
  (Updates.manifest as { version?: string } | undefined)?.version || Constants.expoConfig?.version || '0.0.0';
