import Constants from 'expo-constants';

export type AppVariant = 'production' | 'testing';

const extra = Constants.expoConfig?.extra ?? {};
const envAppVariant = process.env.APP_VARIANT;
const envVersionSuffix = process.env.APP_VERSION_SUFFIX;

export const appVariant: AppVariant =
  (envAppVariant as AppVariant) || (extra.appVariant as AppVariant) || 'production';

export const defaultRemotePath: string =
  (extra.defaultRemotePath as string) || '/remote.php/dav/files/<username>/.daily-tasks.json';

export const storagePrefix: string = (extra.storagePrefix as string) || 'dailyTasks';

export const appVersionSuffix: string =
  envVersionSuffix || (extra.appVersionSuffix as string) || '';

export const commitHash: string =
  (extra.commitHash as string) || appVersionSuffix || '';

export const appVersion: string =
  Constants.expoConfig?.version || '0.0.0';
