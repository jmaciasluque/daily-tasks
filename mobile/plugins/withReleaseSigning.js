// Expo config plugin: sign Android release builds with the Play upload key
// when its environment variables are present (CI release workflow), and fall
// back to the debug key otherwise (local and PR builds stay installable).
//
//   ANDROID_UPLOAD_KEYSTORE_FILE      absolute path to the .jks
//   ANDROID_UPLOAD_KEYSTORE_PASSWORD
//   ANDROID_UPLOAD_KEY_ALIAS
//   ANDROID_UPLOAD_KEY_PASSWORD
const { withAppBuildGradle } = require('expo/config-plugins');

const MARKER = '// daily-tasks: release signing';

const signingConfig = `
        ${MARKER}
        release {
            if (System.getenv("ANDROID_UPLOAD_KEYSTORE_FILE")) {
                storeFile file(System.getenv("ANDROID_UPLOAD_KEYSTORE_FILE"))
                storePassword System.getenv("ANDROID_UPLOAD_KEYSTORE_PASSWORD")
                keyAlias System.getenv("ANDROID_UPLOAD_KEY_ALIAS")
                keyPassword System.getenv("ANDROID_UPLOAD_KEY_PASSWORD")
            }
        }`;

function addReleaseSigning(gradle) {
  if (gradle.includes(MARKER)) {
    return gradle;
  }
  // 1) declare signingConfigs.release next to the template's debug config
  const withConfig = gradle.replace(/signingConfigs\s*\{/, (m) => `${m}${signingConfig}`);
  if (withConfig === gradle) {
    throw new Error('withReleaseSigning: signingConfigs block not found in app/build.gradle');
  }
  // 2) use it for the release build type when the keystore is provided
  const releaseBlock = /(buildTypes\s*\{[\s\S]*?release\s*\{[\s\S]*?)signingConfig\s+signingConfigs\.debug/;
  if (!releaseBlock.test(withConfig)) {
    throw new Error('withReleaseSigning: release signingConfig not found in app/build.gradle');
  }
  return withConfig.replace(
    releaseBlock,
    '$1signingConfig System.getenv("ANDROID_UPLOAD_KEYSTORE_FILE") ? signingConfigs.release : signingConfigs.debug',
  );
}

module.exports = function withReleaseSigning(config) {
  return withAppBuildGradle(config, (cfg) => {
    cfg.modResults.contents = addReleaseSigning(cfg.modResults.contents);
    return cfg;
  });
};
module.exports.addReleaseSigning = addReleaseSigning;
