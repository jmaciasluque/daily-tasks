import React from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import type { Task } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  task: Task;
  theme: Theme;
  onMoveTop: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
};

export function TaskRow({ task, theme, onMoveTop, onMoveUp, onMoveDown, onToggle, onEdit, onDelete }: Props) {
  return (
    <View style={[styles.row, { borderBottomColor: theme.border }]}>
      <View style={styles.rowText}>
        <Text style={[styles.rowTitle, { color: theme.text }]}>{task.title}</Text>
        <Text style={[styles.rowMeta, { color: theme.muted }]}>{task.duration}m</Text>
      </View>
      <View style={styles.rowActions}>
        <Pressable onPress={onMoveTop} style={[styles.iconButton, { borderColor: theme.border }]}>
          <Text style={{ color: theme.text }}>Top</Text>
        </Pressable>
        <Pressable onPress={onMoveUp} style={[styles.iconButton, { borderColor: theme.border }]}>
          <Text style={{ color: theme.text }}>↑</Text>
        </Pressable>
        <Pressable onPress={onMoveDown} style={[styles.iconButton, { borderColor: theme.border }]}>
          <Text style={{ color: theme.text }}>↓</Text>
        </Pressable>
        <Pressable onPress={onToggle} style={[styles.iconButton, { borderColor: theme.border }]}>
          <Text style={{ color: theme.text }}>↔</Text>
        </Pressable>
        <Pressable onPress={onEdit} style={[styles.iconButton, { borderColor: theme.border }]}>
          <Text style={{ color: theme.text }}>Edit</Text>
        </Pressable>
        <Pressable onPress={onDelete} style={[styles.iconButton, { borderColor: theme.border }]}>
          <Text style={{ color: theme.text }}>Del</Text>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    paddingBottom: 12,
    borderBottomWidth: 1,
  },
  rowText: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  rowTitle: {
    fontSize: 16,
    fontWeight: '600',
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
  iconButton: {
    borderWidth: 1,
    borderRadius: 10,
    paddingHorizontal: 10,
    paddingVertical: 6,
  },
});
