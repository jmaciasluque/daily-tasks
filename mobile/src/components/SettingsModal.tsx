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
  onChangeServerUrl: (value: string) => void;
  onUseLocal: () => void;
  onStartNextcloud: () => void;
  onOpenNextcloud: () => void;
  onFinishNextcloud: () => void;
  onClose: () => void;
};

export function SettingsModal({
  visible,
  config,
  serverUrl,
  busy,
  loginPending,
  theme,
  onChangeServerUrl,
  onUseLocal,
  onStartNextcloud,
  onOpenNextcloud,
  onFinishNextcloud,
  onClose,
}: Props) {
  const nextcloudConfig = config.backend === 'nextcloud' ? config.nextcloud : undefined;

  return (
    <Modal visible={visible} animationType="slide" transparent>
      <View style={styles.overlay}>
        <View style={[styles.card, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
          <Text style={[styles.title, { color: theme.text }]}>Storage Backend</Text>
          <ScrollView contentContainerStyle={{ gap: 12 }}>
            <Text style={[styles.copy, { color: theme.muted }]}>
              {config.backend === 'local'
                ? 'This installation is currently using local-only storage.'
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
  },
  title: {
    fontSize: 18,
    fontWeight: '700',
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
