import { StatusBar } from 'expo-status-bar';
import * as Updates from 'expo-updates';
import * as Notifications from 'expo-notifications';
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  AppState,
  FlatList,
  Pressable,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';

import { TaskRow, TaskEditor, SettingsModal } from './src/components';
import { useTaskData } from './src/hooks/useTaskData';
import { orderedTasks } from './src/services/data';
import { setupNotifications, handleNotificationAction } from './src/services/notifications';
import { getTheme, isLightColor } from './src/theme/themes';
import type { Task, TaskStatus, Settings } from './src/types';
import { appVariant, appVersionSuffix, appVersion, commitHash } from './src/config/env';

const updateId = Updates.updateId ? Updates.updateId.slice(0, 7) : 'bundled';
const updateChannel = Updates.channel ?? 'local';

export default function App() {
  const {
    data,
    settings,
    statusMsg,
    syncing,
    syncFromRemote,
    reloadFromCache,
    addTask,
    editTask,
    deleteTask,
    toggleTaskStatus,
    skipTask,
    moveTask,
    moveTaskToTop,
    cycleTheme,
    updateSettings,
  } = useTaskData();

  const [activeStatus, setActiveStatus] = useState<TaskStatus>('todo');
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [pendingSettings, setPendingSettings] = useState<Settings>(settings);
  const [updateReady, setUpdateReady] = useState(false);

  const theme = useMemo(() => getTheme(data.theme_index), [data.theme_index]);
  const list = orderedTasks(data, activeStatus);
  const statusBarStyle = isLightColor(theme.bg) ? 'dark' : 'light';

  // Request notification permissions and set up action handlers
  useEffect(() => {
    setupNotifications();

    // Handle notification actions (Skip / Mark Done) — fired when user interacts
    // with a notification, whether the app is in foreground or background
    const responseSub = Notifications.addNotificationResponseReceivedListener(async (response) => {
      const changed = await handleNotificationAction(response);
      if (changed) {
        // Reload React state from storage after the background update
        await reloadFromCache();
      }
    });

    // When app returns to foreground, reload in case a notification action
    // ran while the app was backgrounded
    const appStateSub = AppState.addEventListener('change', async (nextState) => {
      if (nextState === 'active') {
        await reloadFromCache();
      }
    });

    return () => {
      responseSub.remove();
      appStateSub.remove();
    };
  }, [reloadFromCache]);

  useEffect(() => {
    const checkUpdates = async () => {
      try {
        if (!Updates.isEnabled) {
          return;
        }
        const update = await Updates.checkForUpdateAsync();
        if (update.isAvailable) {
          await Updates.fetchUpdateAsync();
          setUpdateReady(true);
        }
      } catch {
        // Ignore update errors
      }
    };

    checkUpdates();
  }, []);

  const openAdd = () => {
    setEditingTask(null);
    setIsEditorOpen(true);
  };

  const openEdit = (task: Task) => {
    setEditingTask(task);
    setIsEditorOpen(true);
  };

  const handleSaveTask = (title: string, duration: number, deadline?: string) => {
    if (editingTask) {
      editTask(editingTask.id, title, duration, deadline);
    } else {
      addTask(title, duration, deadline);
    }
    setIsEditorOpen(false);
  };

  const handleDeleteTask = (task: Task) => {
    Alert.alert('Delete task', 'Are you sure?', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: () => deleteTask(task.id),
      },
    ]);
  };

  const openSettings = () => {
    setPendingSettings(settings);
    setIsSettingsOpen(true);
  };

  const handleSaveSettings = async () => {
    await updateSettings(pendingSettings);
    setIsSettingsOpen(false);
    syncFromRemote();
  };

  const todoCount = data.tasks.filter(t => t.status === 'todo').length;
  const doneCount = data.tasks.filter(t => t.status === 'done').length;
  const skippedCount = data.tasks.filter(t => t.status === 'skipped').length;

  return (
    <SafeAreaProvider>
      <SafeAreaView style={[styles.root, { backgroundColor: theme.bg }]} edges={['top', 'bottom']}>
        <StatusBar style={statusBarStyle} />

        {/* Header */}
        <View style={[styles.header, { borderBottomColor: theme.border }]}>
          <View>
            <Text style={[styles.title, { color: theme.text }]}>Daily Tasks</Text>
            <Text style={[styles.subtitle, { color: theme.muted }]}>
              Theme: {theme.name}
            </Text>
          </View>
          <View style={styles.headerActions}>
            <Pressable onPress={cycleTheme} style={[styles.smallButton, { borderColor: theme.border }]}>
              <Text style={{ color: theme.text }}>Theme</Text>
            </Pressable>
            <Pressable onPress={openSettings} style={[styles.smallButton, { borderColor: theme.border }]}>
              <Text style={{ color: theme.text }}>Settings</Text>
            </Pressable>
          </View>
        </View>

        {/* Status Switcher */}
        <View style={styles.switcher}>
          <Pressable
            onPress={() => setActiveStatus('todo')}
            style={[
              styles.switchButton,
              { backgroundColor: activeStatus === 'todo' ? theme.focusBg : theme.panelBg, borderColor: theme.border },
            ]}
          >
            <Text style={{ color: theme.text, fontWeight: '600' }}>To Do ({todoCount})</Text>
          </Pressable>
          <Pressable
            onPress={() => setActiveStatus('done')}
            style={[
              styles.switchButton,
              { backgroundColor: activeStatus === 'done' ? theme.focusBg : theme.panelBg, borderColor: theme.border },
            ]}
          >
            <Text style={{ color: theme.text, fontWeight: '600' }}>Done ({doneCount})</Text>
          </Pressable>
          <Pressable
            onPress={() => setActiveStatus('skipped')}
            style={[
              styles.switchButton,
              { backgroundColor: activeStatus === 'skipped' ? theme.focusBg : theme.panelBg, borderColor: theme.border },
            ]}
          >
            <Text style={{ color: theme.muted, fontWeight: '600' }}>Skipped ({skippedCount})</Text>
          </Pressable>
        </View>

        {/* Task List */}
        <View style={[styles.panel, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
          <FlatList
            data={list}
            keyExtractor={(item) => String(item.id)}
            contentContainerStyle={styles.listContent}
            renderItem={({ item }) => (
              <TaskRow
                task={item}
                theme={theme}
                onMoveTop={() => moveTaskToTop(item)}
                onMoveUp={() => moveTask(item, -1)}
                onMoveDown={() => moveTask(item, 1)}
                onToggle={() => toggleTaskStatus(item)}
                onSkip={() => skipTask(item.id)}
                onEdit={() => openEdit(item)}
                onDelete={() => handleDeleteTask(item)}
              />
            )}
            ListEmptyComponent={
              <Text style={[styles.emptyText, { color: theme.muted }]}>No tasks.</Text>
            }
          />
        </View>

        {/* Footer */}
        <View style={styles.footer}>
          <Pressable onPress={openAdd} style={[styles.primaryButton, { backgroundColor: theme.accent }]}>
            <Text style={styles.primaryButtonText}>Add Task</Text>
          </Pressable>
          <Pressable onPress={() => syncFromRemote()} style={[styles.secondaryButton, { borderColor: theme.border }]}>
            <Text style={{ color: theme.text }}>{syncing ? 'Syncing...' : 'Sync'}</Text>
          </Pressable>
        </View>

        {statusMsg ? <Text style={[styles.status, { color: theme.muted }]}>{statusMsg}</Text> : null}
        <Text style={[styles.status, { color: theme.muted }]}>
          Version {appVersion}{appVariant !== 'production' && appVersionSuffix ? `-${appVersionSuffix.slice(0, 7)}` : ''}
        </Text>
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
              onPress={() => Updates.reloadAsync()}
              style={[styles.smallButton, { borderColor: theme.border }]}
            >
              <Text style={{ color: theme.text }}>Restart</Text>
            </Pressable>
          </View>
        ) : null}

        {/* Modals */}
        <TaskEditor
          visible={isEditorOpen}
          task={editingTask}
          theme={theme}
          onSave={handleSaveTask}
          onClose={() => setIsEditorOpen(false)}
        />

        <SettingsModal
          visible={isSettingsOpen}
          settings={pendingSettings}
          theme={theme}
          onUpdate={setPendingSettings}
          onSave={handleSaveSettings}
          onClose={() => setIsSettingsOpen(false)}
        />
      </SafeAreaView>
    </SafeAreaProvider>
  );
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
  },
  header: {
    paddingHorizontal: 18,
    paddingVertical: 16,
    borderBottomWidth: 1,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    gap: 16,
  },
  headerActions: {
    flexDirection: 'row',
    gap: 8,
  },
  title: {
    fontSize: 24,
    fontWeight: '700',
  },
  subtitle: {
    fontSize: 13,
    marginTop: 4,
  },
  switcher: {
    flexDirection: 'row',
    gap: 8,
    paddingHorizontal: 18,
    paddingTop: 8,
  },
  switchButton: {
    flex: 1,
    borderWidth: 1,
    paddingVertical: 10,
    borderRadius: 12,
    alignItems: 'center',
  },
  panel: {
    flex: 1,
    margin: 18,
    borderWidth: 1,
    borderRadius: 16,
  },
  listContent: {
    padding: 12,
    gap: 10,
  },
  emptyText: {
    textAlign: 'center',
    paddingVertical: 24,
  },
  footer: {
    flexDirection: 'row',
    gap: 12,
    paddingHorizontal: 18,
    paddingBottom: 8,
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
  smallButton: {
    borderWidth: 1,
    borderRadius: 10,
    paddingHorizontal: 10,
    paddingVertical: 6,
  },
  status: {
    textAlign: 'center',
    paddingBottom: 12,
  },
  updateBanner: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    gap: 8,
    paddingBottom: 12,
  },
});
