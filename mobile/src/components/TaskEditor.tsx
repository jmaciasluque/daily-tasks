import React, { useState, useEffect } from 'react';
import { Alert, Modal, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import type { Task } from '../types';
import type { Theme } from '../theme/themes';

// Mon-first display; values are JS weekday numbers (0=Sun … 6=Sat)
const DAY_ENTRIES = [
  { label: 'Mon', value: 1 }, { label: 'Tue', value: 2 }, { label: 'Wed', value: 3 },
  { label: 'Thu', value: 4 }, { label: 'Fri', value: 5 }, { label: 'Sat', value: 6 },
  { label: 'Sun', value: 0 },
] as const;

type Props = {
  visible: boolean;
  task: Task | null;
  theme: Theme;
  onSave: (title: string, duration: number, deadline?: string, visibility?: number[]) => void;
  onClose: () => void;
};

export function TaskEditor({ visible, task, theme, onSave, onClose }: Props) {
  const [title, setTitle] = useState('');
  const [duration, setDuration] = useState('5');
  const [deadline, setDeadline] = useState('');
  const [visibility, setVisibility] = useState<number[]>([]);

  useEffect(() => {
    if (task) {
      setTitle(task.title);
      setDuration(String(task.duration));
      setDeadline(task.deadline ?? '');
      setVisibility(task.visibility ?? []);
    } else {
      setTitle('');
      setDuration('5');
      setDeadline('');
      setVisibility([]);
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

    const trimmedDeadline = deadline.trim();
    if (trimmedDeadline) {
      if (!/^\d{2}:\d{2}$/.test(trimmedDeadline)) {
        Alert.alert('Validation', 'Deadline must be in HH:MM format (e.g. 09:30).');
        return;
      }
      const [h, m] = trimmedDeadline.split(':').map(Number);
      if (h < 0 || h > 23 || m < 0 || m > 59) {
        Alert.alert('Validation', 'Deadline time is out of range.');
        return;
      }
    }

    onSave(trimmedTitle, parsedDuration, trimmedDeadline || undefined, visibility.length > 0 ? visibility : undefined);
  };

  const toggleDay = (day: number) => {
    setVisibility((prev) =>
      prev.includes(day) ? prev.filter((d) => d !== day) : [...prev, day].sort()
    );
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
          <TextInput
            placeholder="Deadline (HH:MM, optional)"
            placeholderTextColor={theme.muted}
            value={deadline}
            onChangeText={setDeadline}
            keyboardType="numbers-and-punctuation"
            style={[styles.input, { color: theme.text, borderColor: theme.border }]}
          />
          <Text style={[styles.hint, { color: theme.muted }]}>
            Set a deadline to receive a daily notification reminder.
          </Text>
          <View>
            <Text style={[styles.hint, { color: theme.muted, marginBottom: 6 }]}>
              Visible on (empty = every day):
            </Text>
            <View style={styles.dayRow}>
              {DAY_ENTRIES.map((entry) => (
                <Pressable
                  key={entry.value}
                  onPress={() => toggleDay(entry.value)}
                  style={[
                    styles.dayButton,
                    {
                      borderColor: theme.border,
                      backgroundColor: visibility.includes(entry.value) ? theme.accent : 'transparent',
                    },
                  ]}
                >
                  <Text style={{
                    fontSize: 12,
                    fontWeight: visibility.includes(entry.value) ? '700' : '400',
                    color: visibility.includes(entry.value) ? '#111111' : theme.text,
                    textAlign: 'center',
                  }}>
                    {entry.label}
                  </Text>
                </Pressable>
              ))}
            </View>
          </View>
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
  hint: {
    fontSize: 12,
    marginTop: -4,
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
  dayRow: {
    flexDirection: 'row',
    gap: 4,
  },
  dayButton: {
    flex: 1,
    borderWidth: 1,
    borderRadius: 8,
    paddingVertical: 6,
    alignItems: 'center',
  },
});
