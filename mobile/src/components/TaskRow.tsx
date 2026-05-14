import React from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import type { Task } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  task: Task;
  theme: Theme;
  drag?: () => void;
  isActive?: boolean;
  hiddenToday?: boolean;
  onToggle?: () => void;
  onSkip?: () => void;
  onEdit: () => void;
  onDelete: () => void;
};

function formatVisibility(visibility?: number[]): string {
  if (!visibility || visibility.length === 0) return 'Every day';
  const names = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  return visibility.map((day) => names[day] ?? String(day)).join(', ');
}

export function TaskRow({ task, theme, drag, isActive, hiddenToday, onToggle, onSkip, onEdit, onDelete }: Props) {
  return (
    <View style={[styles.row, { borderBottomColor: theme.border, backgroundColor: isActive ? theme.focusBg : undefined }]}>
      <View style={styles.rowText}>
        {drag && (
          <Pressable onLongPress={drag} style={styles.dragHandle}>
            <Text style={[styles.dragDots, { color: theme.muted }]}>⠿</Text>
          </Pressable>
        )}
        <View style={styles.taskText}>
          <Text style={[styles.rowTitle, { color: theme.text }]}>{task.title}</Text>
          <Text style={[styles.rowMeta, { color: theme.muted }]}>
            {task.duration}m{task.deadline ? ` · ⏰ ${task.deadline}` : ''}
          </Text>
        </View>
      </View>
      {hiddenToday ? (
        <Text style={[styles.visibilityBadge, { color: theme.muted, borderColor: theme.border }]}>
          Hidden today · {formatVisibility(task.visibility)}
        </Text>
      ) : null}
      <View style={styles.rowActions}>
        {onToggle ? (
          <Pressable onPress={onToggle} style={[styles.iconButton, { borderColor: theme.border }]}>
            <Text style={{ color: theme.text }}>
              {task.status === 'todo' ? '✅' : '↩'}
            </Text>
          </Pressable>
        ) : (
          <View style={[styles.iconButton, styles.iconBadge, { borderColor: theme.border }]}>
            <Text style={{ color: theme.muted }}>
              {task.status === 'todo' ? '✅' : '↩'}
            </Text>
          </View>
        )}
        {task.status === 'todo' && onSkip && (
          <Pressable onPress={onSkip} style={[styles.iconButton, { borderColor: theme.border }]}>
            <Text style={{ color: theme.muted }}>⏭</Text>
          </Pressable>
        )}
        {task.status === 'skipped' && (
          <View style={[styles.iconButton, styles.iconBadge, { borderColor: theme.border }]}>
            <Text style={{ color: theme.muted }}>⏭</Text>
          </View>
        )}
        <Pressable onPress={onEdit} style={[styles.iconButton, { borderColor: theme.border }]}>
          <Text style={{ color: theme.text }}>📝</Text>
        </Pressable>
        <Pressable onPress={onDelete} style={[styles.iconButton, { borderColor: theme.border }]}>
          <Text style={{ color: theme.text }}>🗑️</Text>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    paddingBottom: 12,
    borderBottomWidth: 1,
    borderRadius: 8,
  },
  rowText: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  dragHandle: {
    paddingRight: 8,
    paddingVertical: 4,
    justifyContent: 'center',
  },
  dragDots: {
    fontSize: 20,
  },
  taskText: {
    flex: 1,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  rowTitle: {
    fontSize: 16,
    fontWeight: '600',
    flex: 1,
    marginRight: 8,
  },
  rowMeta: {
    fontSize: 13,
  },
  rowActions: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 6,
    marginTop: 8,
  },
  visibilityBadge: {
    alignSelf: 'flex-start',
    borderWidth: 1,
    borderRadius: 8,
    paddingHorizontal: 8,
    paddingVertical: 4,
    fontSize: 12,
    marginTop: 8,
  },
  iconButton: {
    borderWidth: 1,
    borderRadius: 10,
    paddingHorizontal: 10,
    paddingVertical: 6,
  },
  iconBadge: {
    opacity: 0.4,
  },
});
