const baseConfig = require('./app.json');

const appVariant = process.env.APP_VARIANT || 'production';
const isTesting = appVariant !== 'production';
const versionSuffix = process.env.APP_VERSION_SUFFIX
  ? process.env.APP_VERSION_SUFFIX.slice(0, 7)
  : '';

const baseVersion = baseConfig.expo.version;
const version = isTesting && versionSuffix
  ? `${baseVersion}-${versionSuffix}`
  : baseVersion;

const name = isTesting ? 'Daily Tasks (Test)' : baseConfig.expo.name;
const slug = baseConfig.expo.slug;
const androidPackage = isTesting ? 'com.dailytasks.app.test' : 'com.dailytasks.app';
const iosBundleId = isTesting ? 'com.dailytasks.app.test' : 'com.dailytasks.app';

const defaultRemotePath = isTesting
  ? '/remote.php/dav/files/<username>/.daily-tasks-test.json'
  : '/remote.php/dav/files/<username>/.daily-tasks.json';

const storagePrefix = isTesting ? 'dailyTasksTest' : 'dailyTasks';

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
    extra: {
      ...baseConfig.expo.extra,
      appVariant,
      defaultRemotePath,
      storagePrefix,
      appVersionSuffix: versionSuffix,
    },
  },
};
