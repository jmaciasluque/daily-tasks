import React from 'react';
import { Modal, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';
import type { AppConfig } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  visible: boolean;
  config: AppConfig;
  serverUrl: string;
  busy: boolean;
  loginPending: boolean;
  theme: Theme;
  statusMsg: string;
  appVersion: string;
  appVariant: string;
  appVersionSuffix: string;
  commitHash: string;
  updateId: string;
  updateChannel: string;
  updateReady: boolean;
  onChangeServerUrl: (value: string) => void;
  onUseLocal: () => void;
  onStartNextcloud: () => void;
  onOpenNextcloud: () => void;
  onFinishNextcloud: () => void;
  onStartHostedGoogle: () => void;
  onStartHostedFacebook: () => void;
  onHostedLogout: () => void;
  onRestartForUpdate: () => void;
  onClose: () => void;
};

export function SettingsModal({
  visible,
  config,
  serverUrl,
  busy,
  loginPending,
  theme,
  statusMsg,
  appVersion,
  appVariant,
  appVersionSuffix,
  commitHash,
  updateId,
  updateChannel,
  updateReady,
  onChangeServerUrl,
  onUseLocal,
  onStartNextcloud,
  onOpenNextcloud,
  onFinishNextcloud,
  onStartHostedGoogle,
  onStartHostedFacebook,
  onHostedLogout,
  onRestartForUpdate,
  onClose,
}: Props) {
  const nextcloudConfig = config.backend === 'nextcloud' ? config.nextcloud : undefined;
  const hostedConfig = config.backend === 'hosted' ? config.hosted : undefined;
  const versionLabel = `${appVersion}${appVariant !== 'production' && appVersionSuffix ? `-${appVersionSuffix.slice(0, 7)}` : ''}`;

  return (
    <Modal visible={visible} animationType="slide" transparent>
      <View style={styles.overlay}>
        <View style={[styles.card, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
          <Text style={[styles.title, { color: theme.text }]}>Config</Text>
          <ScrollView contentContainerStyle={{ gap: 12 }}>
            <Text style={[styles.sectionLabel, { color: theme.muted }]}>Storage Backend</Text>
            <Text style={[styles.copy, { color: theme.muted }]}>
              {config.backend === 'local'
                ? 'This installation is currently using local-only storage.'
                : config.backend === 'hosted'
                  ? 'This installation is currently connected to the hosted backend.'
                  : 'This installation is currently connected to Nextcloud.'}
            </Text>

            {nextcloudConfig ? (
              <View style={[styles.summary, { borderColor: theme.border }]}>
                <Text style={[styles.summaryLabel, { color: theme.muted }]}>Server</Text>
                <Text style={{ color: theme.text }}>{nextcloudConfig.baseUrl}</Text>
                <Text style={[styles.summaryLabel, { color: theme.muted }]}>Login</Text>
                <Text style={{ color: theme.text }}>{nextcloudConfig.username}</Text>
              </View>
            ) : null}

            {hostedConfig ? (
              <View style={[styles.summary, { borderColor: theme.border }]}>
                <Text style={[styles.summaryLabel, { color: theme.muted }]}>Hosted API</Text>
                <Text style={{ color: theme.text }}>{hostedConfig.apiUrl}</Text>
                <Text style={[styles.summaryLabel, { color: theme.muted }]}>Account</Text>
                <Text style={{ color: theme.text }}>{hostedConfig.email || 'Connected'}</Text>
                <Pressable onPress={onHostedLogout} style={[styles.secondaryButton, { borderColor: theme.border }]}>
                  <Text style={{ color: theme.text }}>Sign out</Text>
                </Pressable>
              </View>
            ) : null}

            <Pressable
              onPress={onStartHostedGoogle}
              style={[styles.primaryButton, { backgroundColor: theme.accent, opacity: busy ? 0.7 : 1 }]}
              disabled={busy}
            >
              <Text style={styles.primaryButtonText}>Sign in with Google</Text>
            </Pressable>
            <Pressable
              onPress={onStartHostedFacebook}
              style={[styles.secondaryButton, { borderColor: theme.border, opacity: busy ? 0.7 : 1 }]}
              disabled={busy}
            >
              <Text style={{ color: theme.text }}>Sign in with Facebook</Text>
            </Pressable>

            <View style={[styles.divider, { borderTopColor: theme.border }]} />

            <TextInput
              placeholder="https://cloud.example.com"
              placeholderTextColor={theme.muted}
              value={serverUrl}
              onChangeText={onChangeServerUrl}
              autoCapitalize="none"
              keyboardType="url"
              style={[styles.input, { color: theme.text, borderColor: theme.border }]}
            />

            {!loginPending ? (
              <Pressable
                onPress={onStartNextcloud}
                style={[styles.primaryButton, { backgroundColor: theme.accent, opacity: busy ? 0.7 : 1 }]}
                disabled={busy}
              >
                <Text style={styles.primaryButtonText}>
                  {config.backend === 'nextcloud' ? 'Reconnect Nextcloud' : 'Connect Nextcloud'}
                </Text>
              </Pressable>
            ) : (
              <View style={styles.pendingActions}>
                <Text style={[styles.copy, { color: theme.muted }]}>
                  Finish the login in Nextcloud, then come back here to complete the connection.
                </Text>
                <Pressable
                  onPress={onOpenNextcloud}
                  style={[styles.secondaryButton, { borderColor: theme.border }]}
                >
                  <Text style={{ color: theme.text }}>Open Nextcloud</Text>
                </Pressable>
                <Pressable
                  onPress={onFinishNextcloud}
                  style={[styles.primaryButton, { backgroundColor: theme.accent, opacity: busy ? 0.7 : 1 }]}
                  disabled={busy}
                >
                  <Text style={styles.primaryButtonText}>Finish Connection</Text>
                </Pressable>
              </View>
            )}

            <View style={[styles.divider, { borderTopColor: theme.border }]} />

            <Text style={[styles.sectionLabel, { color: theme.muted }]}>Status</Text>
            <Text style={[styles.status, { color: theme.muted }]}>
              Backend: {config.backend === 'hosted' ? 'Hosted' : config.backend === 'nextcloud' ? 'Nextcloud' : 'Local only'}
            </Text>
            {statusMsg ? <Text style={[styles.status, { color: theme.muted }]}>{statusMsg}</Text> : null}

            <Text style={[styles.sectionLabel, { color: theme.muted }]}>Build</Text>
            <Text style={[styles.status, { color: theme.muted }]}>Version {versionLabel}</Text>
            {commitHash ? (
              <Text style={[styles.status, { color: theme.muted }]}>Commit {commitHash.slice(0, 7)}</Text>
            ) : null}
            <Text style={[styles.status, { color: theme.muted }]}>Update {updateId} · {updateChannel}</Text>
            {appVariant !== 'production' ? (
              <Text style={[styles.status, { color: theme.muted }]}>Test build</Text>
            ) : null}
            {updateReady ? (
              <View style={styles.updateBanner}>
                <Text style={[styles.status, { color: theme.muted }]}>Update available</Text>
                <Pressable
                  onPress={onRestartForUpdate}
                  style={[styles.secondaryButton, { borderColor: theme.border }]}
                >
                  <Text style={{ color: theme.text }}>Restart</Text>
                </Pressable>
              </View>
            ) : null}
          </ScrollView>
          <View style={styles.actions}>
            <Pressable onPress={onUseLocal} style={[styles.secondaryButton, { borderColor: theme.border }]}>
              <Text style={{ color: theme.text }}>Use Local Only</Text>
            </Pressable>
            <Pressable onPress={onClose} style={[styles.secondaryButton, { borderColor: theme.border }]}>
              <Text style={{ color: theme.text }}>Close</Text>
            </Pressable>
          </View>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    justifyContent: 'center',
    padding: 18,
    backgroundColor: 'rgba(0,0,0,0.35)',
  },
  card: {
    borderWidth: 1,
    borderRadius: 16,
    padding: 18,
    gap: 12,
    maxHeight: '90%',
  },
  title: {
    fontSize: 18,
    fontWeight: '700',
  },
  sectionLabel: {
    fontSize: 12,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
    marginTop: 4,
  },
  copy: {
    lineHeight: 20,
  },
  summary: {
    borderWidth: 1,
    borderRadius: 12,
    padding: 12,
    gap: 6,
  },
  summaryLabel: {
    fontSize: 12,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  input: {
    borderWidth: 1,
    borderRadius: 12,
    paddingHorizontal: 12,
    paddingVertical: 10,
  },
  pendingActions: {
    gap: 12,
  },
  divider: {
    borderTopWidth: 1,
    marginTop: 8,
    marginBottom: 4,
  },
  status: {
    fontSize: 13,
  },
  updateBanner: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    marginTop: 4,
  },
  actions: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    gap: 12,
    marginTop: 8,
  },
  primaryButton: {
    borderRadius: 12,
    paddingHorizontal: 16,
    paddingVertical: 12,
    alignItems: 'center',
  },
  primaryButtonText: {
    color: '#111111',
    fontWeight: '700',
  },
  secondaryButton: {
    borderWidth: 1,
    borderRadius: 12,
    paddingHorizontal: 16,
    paddingVertical: 12,
    alignItems: 'center',
  },
});
