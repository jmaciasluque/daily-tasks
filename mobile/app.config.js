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

// Play requires a strictly increasing integer versionCode. Derive it from the
// semantic version (major*1_000_000 + minor*1_000 + patch) unless CI overrides
// it, so every VERSION bump yields a higher code.
function versionCodeFrom(semver) {
  const [major, minor, patch] = semver.split('.').map((part) => parseInt(part, 10) || 0);
  return major * 1000000 + minor * 1000 + patch;
}
const versionCode = parseInt(process.env.ANDROID_VERSION_CODE || '', 10) || versionCodeFrom(baseVersion);

// Production and dev builds install side by side, so each needs its own
// application ID and its own URL scheme (otherwise Android shows a chooser
// when the OAuth redirect comes back).
const name = isTesting ? 'Daily Tasks (Dev)' : baseConfig.expo.name;
const slug = baseConfig.expo.slug;
const applicationId = isTesting ? 'be.maclug.dailytasks.dev' : 'be.maclug.dailytasks';
const scheme = isTesting ? 'daily-tasks-dev' : 'daily-tasks';

const defaultRemotePath = isTesting
  ? '/remote.php/dav/files/<username>/.daily-tasks-test.json'
  : '/remote.php/dav/files/<username>/.daily-tasks.json';

const hostedApiUrl = process.env.HOSTED_API_URL || 'https://daily-tasks-api.fly.dev';
const storagePrefix = isTesting ? 'dailyTasksTest' : 'dailyTasks';
const projectId = baseConfig.expo?.extra?.eas?.projectId;
const updatesUrl = projectId ? `https://u.expo.dev/${projectId}` : undefined;

module.exports = {
  expo: {
    ...baseConfig.expo,
    name,
    slug,
    scheme,
    version,
    ios: {
      ...baseConfig.expo.ios,
      bundleIdentifier: applicationId,
    },
    android: {
      ...baseConfig.expo.android,
      package: applicationId,
      versionCode,
    },
    plugins: [
      ...(baseConfig.expo.plugins || []),
      // Dev builds may talk to a local server over plain HTTP; production
      // builds never allow cleartext traffic.
      ['expo-build-properties', { android: { usesCleartextTraffic: isTesting } }],
      './plugins/withReleaseSigning',
    ],
    updates: updatesUrl ? { url: updatesUrl } : baseConfig.expo.updates,
    runtimeVersion: runtimeVersion,
    extra: {
      ...baseConfig.expo.extra,
      appVariant,
      defaultRemotePath,
      hostedApiUrl,
      scheme,
      storagePrefix,
      appVersionSuffix: versionSuffix,
      commitHash,
    },
  },
};
