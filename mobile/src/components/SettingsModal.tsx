import React from 'react';
import { Modal, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';
import type { Settings } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  visible: boolean;
  settings: Settings;
  theme: Theme;
  onUpdate: (settings: Settings) => void;
  onSave: () => void;
  onClose: () => void;
};

export function SettingsModal({ visible, settings, theme, onUpdate, onSave, onClose }: Props) {
  return (
    <Modal visible={visible} animationType="slide" transparent>
      <View style={styles.overlay}>
        <View style={[styles.card, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
          <Text style={[styles.title, { color: theme.text }]}>Nextcloud WebDAV</Text>
          <ScrollView contentContainerStyle={{ gap: 12 }}>
            <TextInput
              placeholder="Base URL (https://cloud.example.com)"
              placeholderTextColor={theme.muted}
              value={settings.baseUrl}
              onChangeText={(value) => onUpdate({ ...settings, baseUrl: value })}
              autoCapitalize="none"
              style={[styles.input, { color: theme.text, borderColor: theme.border }]}
            />
            <TextInput
              placeholder="Username"
              placeholderTextColor={theme.muted}
              value={settings.username}
              onChangeText={(value) => onUpdate({ ...settings, username: value })}
              autoCapitalize="none"
              style={[styles.input, { color: theme.text, borderColor: theme.border }]}
            />
            <TextInput
              placeholder="App password"
              placeholderTextColor={theme.muted}
              value={settings.password}
              onChangeText={(value) => onUpdate({ ...settings, password: value })}
              autoCapitalize="none"
              secureTextEntry
              style={[styles.input, { color: theme.text, borderColor: theme.border }]}
            />
            <TextInput
              placeholder="Remote path (/remote.php/dav/files/user/.daily-tasks.json)"
              placeholderTextColor={theme.muted}
              value={settings.remotePath}
              onChangeText={(value) => onUpdate({ ...settings, remotePath: value })}
              autoCapitalize="none"
              style={[styles.input, { color: theme.text, borderColor: theme.border }]}
            />
          </ScrollView>
          <View style={styles.actions}>
            <Pressable onPress={onClose} style={[styles.secondaryButton, { borderColor: theme.border }]}>
              <Text style={{ color: theme.text }}>Close</Text>
            </Pressable>
            <Pressable onPress={onSave} style={[styles.primaryButton, { backgroundColor: theme.accent }]}>
              <Text style={styles.primaryButtonText}>Save</Text>
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
  input: {
    borderWidth: 1,
    borderRadius: 12,
    paddingHorizontal: 12,
    paddingVertical: 10,
  },
  actions: {
    flexDirection: 'row',
    justifyContent: 'flex-end',
    gap: 12,
    marginTop: 8,
  },
  primaryButton: {
    borderRadius: 12,
    paddingHorizontal: 16,
    paddingVertical: 12,
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
  },
});
