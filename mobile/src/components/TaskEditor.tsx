import React, { useState, useEffect } from 'react';
import { Alert, Modal, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import type { Task } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  visible: boolean;
  task: Task | null;
  theme: Theme;
  onSave: (title: string, duration: number) => void;
  onClose: () => void;
};

export function TaskEditor({ visible, task, theme, onSave, onClose }: Props) {
  const [title, setTitle] = useState('');
  const [duration, setDuration] = useState('5');

  useEffect(() => {
    if (task) {
      setTitle(task.title);
      setDuration(String(task.duration));
    } else {
      setTitle('');
      setDuration('5');
    }
  }, [task, visible]);

  const handleSave = () => {
    const trimmedTitle = title.trim();
    const parsedDuration = Number.parseInt(duration, 10);

    if (!trimmedTitle) {
      Alert.alert('Validation', 'Title cannot be empty.');
      return;
    }
    if (!Number.isFinite(parsedDuration) || parsedDuration <= 0) {
      Alert.alert('Validation', 'Duration must be a positive integer.');
      return;
    }

    onSave(trimmedTitle, parsedDuration);
  };

  return (
    <Modal visible={visible} animationType="slide" transparent>
      <View style={styles.overlay}>
        <View style={[styles.card, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
          <Text style={[styles.title, { color: theme.text }]}>
            {task ? 'Edit Task' : 'Add Task'}
          </Text>
          <TextInput
            placeholder="Title"
            placeholderTextColor={theme.muted}
            value={title}
            onChangeText={setTitle}
            style={[styles.input, { color: theme.text, borderColor: theme.border }]}
          />
          <TextInput
            placeholder="Duration (minutes)"
            placeholderTextColor={theme.muted}
            value={duration}
            onChangeText={setDuration}
            keyboardType="number-pad"
            style={[styles.input, { color: theme.text, borderColor: theme.border }]}
          />
          <View style={styles.actions}>
            <Pressable onPress={onClose} style={[styles.secondaryButton, { borderColor: theme.border }]}>
              <Text style={{ color: theme.text }}>Cancel</Text>
            </Pressable>
            <Pressable onPress={handleSave} style={[styles.primaryButton, { backgroundColor: theme.accent }]}>
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
