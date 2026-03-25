const fs = require('fs');
const path = require('path');
const baseConfig = require('./app.json');

const appVariant = process.env.APP_VARIANT || 'production';
const isTesting = appVariant !== 'production';
const commitFromEnv = process.env.APP_VERSION_SUFFIX
  || process.env.EAS_BUILD_GIT_COMMIT_HASH
  || process.env.GITHUB_SHA
  || '';
const versionSuffix = commitFromEnv ? commitFromEnv.slice(0, 7) : '';

// Read version from the single source of truth at repo root
const versionFile = path.resolve(__dirname, '..', 'VERSION');
const baseVersion = fs.existsSync(versionFile)
  ? fs.readFileSync(versionFile, 'utf8').trim()
  : baseConfig.expo.version;
const version = isTesting && versionSuffix
  ? `${baseVersion}-${versionSuffix}`
  : baseVersion;
const runtimeVersion = baseVersion;
const commitHash = versionSuffix;

const name = isTesting ? 'Daily Tasks (Test)' : baseConfig.expo.name;
const slug = baseConfig.expo.slug;
const androidPackage = isTesting ? 'com.dailytasks.app.test' : 'com.dailytasks.app';
const iosBundleId = isTesting ? 'com.dailytasks.app.test' : 'com.dailytasks.app';

const defaultRemotePath = isTesting
  ? '/remote.php/dav/files/<username>/.daily-tasks-test.json'
  : '/remote.php/dav/files/<username>/.daily-tasks.json';

const storagePrefix = isTesting ? 'dailyTasksTest' : 'dailyTasks';
const projectId = baseConfig.expo?.extra?.eas?.projectId;
const updatesUrl = projectId ? `https://u.expo.dev/${projectId}` : undefined;

module.exports = {
  expo: {
    ...baseConfig.expo,
    name,
    slug,
    version,
    ios: {
      ...baseConfig.expo.ios,
      bundleIdentifier: iosBundleId,
    },
    android: {
      ...baseConfig.expo.android,
      package: androidPackage,
    },
    updates: updatesUrl ? { url: updatesUrl } : baseConfig.expo.updates,
    runtimeVersion: runtimeVersion,
    extra: {
      ...baseConfig.expo.extra,
      appVariant,
      defaultRemotePath,
      storagePrefix,
      appVersionSuffix: versionSuffix,
      commitHash,
    },
  },
};
