import { StatusBar } from 'expo-status-bar';
import * as Updates from 'expo-updates';
import * as Notifications from 'expo-notifications';
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  AppState,
  FlatList,
  Linking,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import DraggableFlatList from 'react-native-draggable-flatlist';

import { StatsScreen, TaskRow, TaskEditor, SettingsModal } from './src/components';
import { useTaskData } from './src/hooks/useTaskData';
import { orderedTasks } from './src/services/data';
import { buildStatsSummary } from './src/services/history';
import { setupNotifications, handleNotificationAction, scheduleTestNotification } from './src/services/notifications';
import { pollNextcloudLogin, startNextcloudLogin, type LoginFlowSession } from './src/services/backend_webdav';
import { getTheme, isLightColor } from './src/theme/themes';
import type { Task, TaskStatus } from './src/types';
import { appVariant, appVersionSuffix, appVersion, commitHash } from './src/config/env';
import type { StatsPeriod } from './src/components/StatsScreen';

const updateId = Updates.updateId ? Updates.updateId.slice(0, 7) : 'bundled';
const updateChannel = Updates.channel ?? 'local';
type Screen = 'tasks' | 'stats';

function formatDateInput(date: Date): string {
  return date.toISOString().slice(0, 10);
}

function rangeForPeriod(period: StatsPeriod): { from: string; to: string } {
  const end = new Date();
  if (period === 'today') {
    const today = formatDateInput(end);
    return { from: today, to: today };
  }
  const days = period === '7d' ? 7 : period === '90d' ? 90 : period === '365d' ? 365 : 30;
  const start = new Date(end);
  start.setDate(end.getDate() - (days - 1));
  return {
    from: formatDateInput(start),
    to: formatDateInput(end),
  };
}

export default function App() {
  const {
    data,
    history,
    config,
    settings,
    backendConfigured,
    nextcloudConfigured,
    statusMsg,
    syncing,
    syncFromRemote,
    reloadFromCache,
    addTask,
    editTask,
    deleteTask,
    toggleTaskStatus,
    skipTask,
    reorderTasks,
    cycleTheme,
    chooseLocalBackend,
    saveNextcloudSettings,
  } = useTaskData();

  const [screen, setScreen] = useState<Screen>('tasks');
  const [activeStatus, setActiveStatus] = useState<TaskStatus>('todo');
  const [statsPeriod, setStatsPeriod] = useState<StatsPeriod>('today');
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [updateReady, setUpdateReady] = useState(false);
  const [serverUrlInput, setServerUrlInput] = useState('');
  const [loginSession, setLoginSession] = useState<LoginFlowSession | null>(null);
  const [backendBusy, setBackendBusy] = useState(false);

  const theme = useMemo(() => getTheme(data.theme_index), [data.theme_index]);
  const list = orderedTasks(data, activeStatus);
  const statusBarStyle = isLightColor(theme.bg) ? 'dark' : 'light';
  const statsRange = useMemo(() => rangeForPeriod(statsPeriod), [statsPeriod]);
  const stats = useMemo(
    () => buildStatsSummary(history, data, statsRange.from, statsRange.to),
    [data, history, statsRange.from, statsRange.to],
  );

  useEffect(() => {
    if (!serverUrlInput && settings.baseUrl) {
      setServerUrlInput(settings.baseUrl);
    }
  }, [serverUrlInput, settings.baseUrl]);

  useEffect(() => {
    const consumeNotificationResponse = async (response: Notifications.NotificationResponse) => {
      const changed = await handleNotificationAction(response);
      if (changed) {
        await reloadFromCache();
      }

      if (response.actionIdentifier === 'skip' || response.actionIdentifier === 'done') {
        await Notifications.clearLastNotificationResponseAsync().catch(() => {});
      }
    };

    setupNotifications();

    Notifications.getLastNotificationResponseAsync().then(async (response) => {
      if (response) {
        await consumeNotificationResponse(response);
      }
    });

    const responseSub = Notifications.addNotificationResponseReceivedListener(async (response) => {
      await consumeNotificationResponse(response);
    });

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

  const handleChooseLocalBackend = useCallback(async () => {
    setBackendBusy(true);
    try {
      setLoginSession(null);
      await chooseLocalBackend();
      setIsSettingsOpen(false);
    } finally {
      setBackendBusy(false);
    }
  }, [chooseLocalBackend]);

  const handleStartNextcloudLogin = useCallback(async () => {
    const serverUrl = serverUrlInput.trim() || settings.baseUrl;
    if (!serverUrl) {
      Alert.alert('Server URL required', 'Enter your Nextcloud server URL first.');
      return;
    }

    setBackendBusy(true);
    try {
      const session = await startNextcloudLogin(serverUrl);
      setLoginSession(session);
      setServerUrlInput(session.serverUrl);
      await Linking.openURL(session.loginUrl);
    } catch (err) {
      Alert.alert('Connection error', (err as Error).message);
    } finally {
      setBackendBusy(false);
    }
  }, [serverUrlInput, settings.baseUrl]);

  const handleOpenPendingLogin = useCallback(async () => {
    if (!loginSession) {
      return;
    }
    await Linking.openURL(loginSession.loginUrl).catch(() => {});
  }, [loginSession]);

  const handleFinishNextcloudLogin = useCallback(async () => {
    if (!loginSession) {
      return;
    }

    setBackendBusy(true);
    try {
      const nextcloudSettings = await pollNextcloudLogin(loginSession);
      if (!nextcloudSettings) {
        Alert.alert(
          'Still waiting',
          'Finish the approval in Nextcloud first, then come back here and try again.',
        );
        return;
      }

      await saveNextcloudSettings(nextcloudSettings);
      setLoginSession(null);
      setServerUrlInput(nextcloudSettings.baseUrl);
      setIsSettingsOpen(false);
    } catch (err) {
      Alert.alert('Connection error', (err as Error).message);
    } finally {
      setBackendBusy(false);
    }
  }, [loginSession, saveNextcloudSettings]);

  const openAdd = () => {
    setEditingTask(null);
    setIsEditorOpen(true);
  };

  const openEdit = (task: Task) => {
    setEditingTask(task);
    setIsEditorOpen(true);
  };

  const handleSaveTask = (title: string, duration: number, deadline?: string, visibility?: number[]) => {
    if (editingTask) {
      editTask(editingTask.id, title, duration, deadline, visibility);
    } else {
      addTask(title, duration, deadline, visibility);
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

  const handleTestNotification = async () => {
    const task = orderedTasks(data, 'todo')[0];
    if (!task) {
      Alert.alert('No todo task', 'Add or reopen a todo task first.');
      return;
    }

    const granted = await setupNotifications();
    if (!granted) {
      Alert.alert('Notifications disabled', 'Allow notifications for Daily Tasks first.');
      return;
    }

    try {
      await scheduleTestNotification(task);
      Alert.alert(
        'Test notification scheduled',
        `A test notification for "${task.title}" will appear in a few seconds.`,
      );
    } catch (err) {
      Alert.alert('Notification error', (err as Error).message);
    }
  };

  const openSettings = () => {
    setServerUrlInput(settings.baseUrl);
    setIsSettingsOpen(true);
  };

  const todoCount = orderedTasks(data, 'todo').length;
  const doneCount = orderedTasks(data, 'done').length;
  const skippedCount = orderedTasks(data, 'skipped').length;

  const renderSetupScreen = () => (
    <View style={styles.setupScreen}>
      <View style={[styles.setupCard, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
        <Text style={[styles.title, { color: theme.text }]}>Choose a Backend</Text>
        <Text style={[styles.subtitle, { color: theme.muted }]}>
          Daily Tasks now blocks first use until you choose how this installation should store and sync data.
        </Text>

        <Pressable
          onPress={handleChooseLocalBackend}
          style={[styles.secondaryButton, { borderColor: theme.border, opacity: backendBusy ? 0.7 : 1 }]}
          disabled={backendBusy}
        >
          <Text style={{ color: theme.text }}>Use Local Only</Text>
        </Pressable>

        <Text style={[styles.setupLabel, { color: theme.muted }]}>Connect Nextcloud</Text>
        <TextInput
          placeholder="https://cloud.example.com"
          placeholderTextColor={theme.muted}
          value={serverUrlInput}
          onChangeText={setServerUrlInput}
          autoCapitalize="none"
          keyboardType="url"
          style={[styles.input, { color: theme.text, borderColor: theme.border }]}
        />
        {!loginSession ? (
          <Pressable
            onPress={handleStartNextcloudLogin}
            style={[styles.primaryButton, { backgroundColor: theme.accent, opacity: backendBusy ? 0.7 : 1 }]}
            disabled={backendBusy}
          >
            <Text style={styles.primaryButtonText}>Connect Nextcloud</Text>
          </Pressable>
        ) : (
          <View style={styles.setupActions}>
            <Pressable onPress={handleOpenPendingLogin} style={[styles.secondaryButton, { borderColor: theme.border }]}>
              <Text style={{ color: theme.text }}>Open Nextcloud</Text>
            </Pressable>
            <Pressable
              onPress={handleFinishNextcloudLogin}
              style={[styles.primaryButton, { backgroundColor: theme.accent, opacity: backendBusy ? 0.7 : 1 }]}
              disabled={backendBusy}
            >
              <Text style={styles.primaryButtonText}>Finish Connection</Text>
            </Pressable>
          </View>
        )}

        {statusMsg ? <Text style={[styles.setupStatus, { color: theme.muted }]}>{statusMsg}</Text> : null}
      </View>
    </View>
  );

  return (
    <GestureHandlerRootView style={styles.gestureRoot}>
      <SafeAreaProvider>
        <SafeAreaView style={[styles.root, { backgroundColor: theme.bg }]} edges={['top', 'bottom']}>
          <StatusBar style={statusBarStyle} />

          {!backendConfigured ? (
            renderSetupScreen()
          ) : (
            <>
              <View style={[styles.header, { borderBottomColor: theme.border }]}>
                <View>
                  <Text style={[styles.title, { color: theme.text }]}>Daily Tasks</Text>
                  <Text style={[styles.subtitle, { color: theme.muted }]}>
                    Theme: {theme.name}
                  </Text>
                </View>
                <View style={styles.headerActions}>
                  <Pressable onPress={() => setScreen('tasks')} style={[styles.smallButton, { borderColor: theme.border, backgroundColor: screen === 'tasks' ? theme.focusBg : theme.panelBg }]}>
                    <Text style={{ color: theme.text }}>Tasks</Text>
                  </Pressable>
                  <Pressable onPress={() => setScreen('stats')} style={[styles.smallButton, { borderColor: theme.border, backgroundColor: screen === 'stats' ? theme.focusBg : theme.panelBg }]}>
                    <Text style={{ color: theme.text }}>Stats</Text>
                  </Pressable>
                  <Pressable onPress={cycleTheme} style={[styles.smallButton, { borderColor: theme.border }]}>
                    <Text style={{ color: theme.text }}>Theme</Text>
                  </Pressable>
                  <Pressable onPress={openSettings} style={[styles.smallButton, { borderColor: theme.border }]}>
                    <Text style={{ color: theme.text }}>Backend</Text>
                  </Pressable>
                </View>
              </View>

              {screen === 'tasks' ? (
                <>
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

                  <View style={[styles.panel, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
                    {activeStatus === 'todo' ? (
                      <DraggableFlatList
                        data={list}
                        keyExtractor={(item) => String(item.id)}
                        contentContainerStyle={styles.listContent}
                        onDragEnd={({ data }) => reorderTasks(data)}
                        renderItem={({ item, drag, isActive }) => (
                          <TaskRow
                            task={item}
                            theme={theme}
                            drag={drag}
                            isActive={isActive}
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
                    ) : (
                      <FlatList
                        data={list}
                        keyExtractor={(item) => String(item.id)}
                        contentContainerStyle={styles.listContent}
                        renderItem={({ item }) => (
                          <TaskRow
                            task={item}
                            theme={theme}
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
                    )}
                  </View>

                  <View style={styles.footer}>
                    <Pressable onPress={openAdd} style={[styles.primaryButton, styles.footerButton, { backgroundColor: theme.accent }]}>
                      <Text style={styles.primaryButtonText}>Add Task</Text>
                    </Pressable>
                    <Pressable onPress={handleTestNotification} style={[styles.secondaryButton, styles.footerButton, { borderColor: theme.border }]}>
                      <Text style={{ color: theme.text }}>Test Notif</Text>
                    </Pressable>
                    {nextcloudConfigured ? (
                      <Pressable onPress={() => syncFromRemote()} style={[styles.secondaryButton, styles.footerButton, { borderColor: theme.border }]}>
                        <Text style={{ color: theme.text }}>{syncing ? 'Syncing...' : 'Sync'}</Text>
                      </Pressable>
                    ) : (
                      <View style={[styles.secondaryButton, styles.footerButton, { borderColor: theme.border }]}>
                        <Text style={{ color: theme.muted }}>Local Only</Text>
                      </View>
                    )}
                  </View>
                </>
              ) : (
                <StatsScreen
                  period={statsPeriod}
                  stats={stats}
                  theme={theme}
                  onSelectPeriod={setStatsPeriod}
                />
              )}

              <Text style={[styles.status, { color: theme.muted }]}>
                Backend: {config.backend === 'nextcloud' ? 'Nextcloud' : 'Local only'}
              </Text>
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

              <TaskEditor
                visible={isEditorOpen}
                task={editingTask}
                theme={theme}
                onSave={handleSaveTask}
                onClose={() => setIsEditorOpen(false)}
              />

              <SettingsModal
                visible={isSettingsOpen}
                config={config}
                serverUrl={serverUrlInput}
                busy={backendBusy}
                loginPending={!!loginSession}
                theme={theme}
                onChangeServerUrl={setServerUrlInput}
                onUseLocal={handleChooseLocalBackend}
                onStartNextcloud={handleStartNextcloudLogin}
                onOpenNextcloud={handleOpenPendingLogin}
                onFinishNextcloud={handleFinishNextcloudLogin}
                onClose={() => setIsSettingsOpen(false)}
              />
            </>
          )}
        </SafeAreaView>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}

const styles = StyleSheet.create({
  gestureRoot: {
    flex: 1,
  },
  root: {
    flex: 1,
  },
  setupScreen: {
    flex: 1,
    justifyContent: 'center',
    padding: 18,
  },
  setupCard: {
    borderWidth: 1,
    borderRadius: 20,
    padding: 20,
    gap: 12,
  },
  setupLabel: {
    marginTop: 8,
    fontSize: 12,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  setupActions: {
    gap: 12,
  },
  setupStatus: {
    textAlign: 'center',
    paddingTop: 6,
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
    flexWrap: 'wrap',
    justifyContent: 'flex-end',
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
  footerButton: {
    flex: 1,
    alignItems: 'center',
  },
  input: {
    borderWidth: 1,
    borderRadius: 12,
    paddingHorizontal: 12,
    paddingVertical: 10,
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
