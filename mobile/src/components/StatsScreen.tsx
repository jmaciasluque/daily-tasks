import React, { useMemo } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { Theme } from '../theme/themes';
import type { StatsSummary } from '../types';

export type StatsPeriod = '7d' | '30d' | '90d' | '365d';

type StatsScreenProps = {
  period: StatsPeriod;
  stats: StatsSummary;
  theme: Theme;
  onSelectPeriod: (period: StatsPeriod) => void;
};

function formatShortDate(value: string): string {
  return new Date(`${value}T00:00:00`).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function formatPercent(value: number): string {
  return `${Math.round(value * 100)}%`;
}

function formatDuration(minutes: number): string {
  if (minutes < 60) {
    return `${minutes}m`;
  }

  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  return mins === 0 ? `${hours}h` : `${hours}h ${mins}m`;
}

export function StatsScreen({ period, stats, theme, onSelectPeriod }: StatsScreenProps) {
  const statsTotals = stats.done_count + stats.skipped_count + stats.todo_count;
  const maxDailyTasks = useMemo(
    () => Math.max(...stats.daily.map((day) => day.task_count), 1),
    [stats.daily],
  );

  return (
    <ScrollView
      style={styles.scroll}
      contentContainerStyle={styles.content}
      showsVerticalScrollIndicator={false}
    >
      <View style={[styles.heroCard, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
        <Text style={[styles.heading, { color: theme.text }]}>Stats</Text>
        <Text style={[styles.subheading, { color: theme.muted }]}>
          {stats.from} to {stats.to}
        </Text>
        <View style={styles.periodRow}>
          {(['7d', '30d', '90d', '365d'] as const).map((candidate) => (
            <Pressable
              key={candidate}
              onPress={() => onSelectPeriod(candidate)}
              style={[
                styles.periodButton,
                {
                  backgroundColor: period === candidate ? theme.focusBg : theme.bg,
                  borderColor: theme.border,
                },
              ]}
            >
              <Text style={{ color: theme.text, fontWeight: period === candidate ? '700' : '500' }}>
                {candidate}
              </Text>
            </Pressable>
          ))}
        </View>
      </View>

      <View style={styles.summaryGrid}>
        {[
          { label: 'Recorded Days', value: String(stats.recorded_days) },
          { label: 'Completion Rate', value: formatPercent(stats.completion_rate) },
          { label: 'Done Time', value: formatDuration(stats.done_duration) },
          { label: 'Skipped Time', value: formatDuration(stats.skipped_duration) },
        ].map((card) => (
          <View
            key={card.label}
            style={[styles.summaryCard, { backgroundColor: theme.panelBg, borderColor: theme.border }]}
          >
            <Text style={[styles.summaryLabel, { color: theme.muted }]}>{card.label}</Text>
            <Text style={[styles.summaryValue, { color: theme.text }]}>{card.value}</Text>
          </View>
        ))}
      </View>

      <View style={[styles.panel, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
        <Text style={[styles.panelTitle, { color: theme.text }]}>Status Mix</Text>
        <Text style={[styles.panelCopy, { color: theme.muted }]}>
          All recorded task snapshots in this range
        </Text>
        <View style={[styles.mixBar, { backgroundColor: theme.bg, borderColor: theme.border }]}>
          {statsTotals === 0 ? (
            <View style={[styles.mixSegment, { flex: 1, backgroundColor: theme.border }]} />
          ) : (
            <>
              {stats.done_count > 0 ? (
                <View style={[styles.mixSegment, { flex: stats.done_count, backgroundColor: theme.accent }]} />
              ) : null}
              {stats.skipped_count > 0 ? (
                <View style={[styles.mixSegment, { flex: stats.skipped_count, backgroundColor: theme.focusBorder }]} />
              ) : null}
              {stats.todo_count > 0 ? (
                <View style={[styles.mixSegment, { flex: stats.todo_count, backgroundColor: theme.muted }]} />
              ) : null}
            </>
          )}
        </View>
        <View style={styles.mixLegend}>
          {[
            ['Done', stats.done_count, theme.accent],
            ['Skipped', stats.skipped_count, theme.focusBorder],
            ['Todo', stats.todo_count, theme.muted],
          ].map(([label, value, color]) => (
            <View key={label} style={styles.mixLegendRow}>
              <View style={styles.mixLegendLabel}>
                <View style={[styles.mixLegendDot, { backgroundColor: color as string }]} />
                <Text style={{ color: theme.text }}>{label}</Text>
              </View>
              <Text style={{ color: theme.text, fontWeight: '700' }}>{String(value)}</Text>
            </View>
          ))}
        </View>
      </View>

      <View style={[styles.panel, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
        <Text style={[styles.panelTitle, { color: theme.text }]}>Daily Histogram</Text>
        <Text style={[styles.panelCopy, { color: theme.muted }]}>
          Done, skipped, and remaining tasks captured each day
        </Text>
        {stats.daily.length === 0 ? (
          <Text style={[styles.emptyCopy, { color: theme.muted }]}>
            No history yet. Use the app for a day to populate this view.
          </Text>
        ) : (
          <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.histogramRow}>
            {stats.daily.map((day) => (
              <View key={day.date} style={styles.histogramColumn}>
                <View style={[styles.histogramTrack, { backgroundColor: theme.bg, borderColor: theme.border }]}>
                  <View
                    style={[
                      styles.histogramSegment,
                      { height: `${(day.done_count / maxDailyTasks) * 100}%`, backgroundColor: theme.accent },
                    ]}
                  />
                  <View
                    style={[
                      styles.histogramSegment,
                      {
                        height: `${(day.skipped_count / maxDailyTasks) * 100}%`,
                        backgroundColor: theme.focusBorder,
                      },
                    ]}
                  />
                  <View
                    style={[
                      styles.histogramSegment,
                      { height: `${(day.todo_count / maxDailyTasks) * 100}%`, backgroundColor: theme.muted },
                    ]}
                  />
                </View>
                <Text style={[styles.histogramLabel, { color: theme.muted }]}>{formatShortDate(day.date)}</Text>
              </View>
            ))}
          </ScrollView>
        )}
      </View>

      <View style={[styles.panel, { backgroundColor: theme.panelBg, borderColor: theme.border }]}>
        <Text style={[styles.panelTitle, { color: theme.text }]}>Task Frequency</Text>
        <Text style={[styles.panelCopy, { color: theme.muted }]}>
          Which tasks recur most often and how consistently they get done
        </Text>
        {stats.tasks.length === 0 ? (
          <Text style={[styles.emptyCopy, { color: theme.muted }]}>
            No task history recorded yet.
          </Text>
        ) : (
          <View style={styles.frequencyList}>
            {stats.tasks.slice(0, 8).map((task) => (
              <View
                key={task.task_id}
                style={[styles.frequencyCard, { backgroundColor: theme.bg, borderColor: theme.border }]}
              >
                <View style={styles.frequencyHeader}>
                  <Text style={[styles.frequencyTitle, { color: theme.text }]} numberOfLines={1}>
                    {task.title}
                  </Text>
                  <Text style={[styles.frequencyDays, { color: theme.muted }]}>
                    {task.recorded_days} days
                  </Text>
                </View>
                <View style={styles.frequencyStats}>
                  <View style={styles.frequencyStat}>
                    <Text style={[styles.frequencyStatLabel, { color: theme.muted }]}>Done</Text>
                    <Text style={[styles.frequencyStatValue, { color: theme.text }]}>{task.done_days}</Text>
                  </View>
                  <View style={styles.frequencyStat}>
                    <Text style={[styles.frequencyStatLabel, { color: theme.muted }]}>Skipped</Text>
                    <Text style={[styles.frequencyStatValue, { color: theme.text }]}>{task.skipped_days}</Text>
                  </View>
                  <View style={styles.frequencyStat}>
                    <Text style={[styles.frequencyStatLabel, { color: theme.muted }]}>Completion</Text>
                    <Text style={[styles.frequencyStatValue, { color: theme.text }]}>
                      {formatPercent(task.completion_rate)}
                    </Text>
                  </View>
                </View>
              </View>
            ))}
          </View>
        )}
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  scroll: {
    flex: 1,
  },
  content: {
    padding: 18,
    gap: 14,
  },
  heroCard: {
    borderWidth: 1,
    borderRadius: 22,
    padding: 18,
    gap: 10,
  },
  heading: {
    fontSize: 24,
    fontWeight: '800',
  },
  subheading: {
    fontSize: 14,
  },
  periodRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
  },
  periodButton: {
    borderWidth: 1,
    borderRadius: 999,
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
  summaryGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 12,
  },
  summaryCard: {
    width: '47%',
    borderWidth: 1,
    borderRadius: 18,
    padding: 16,
    gap: 8,
  },
  summaryLabel: {
    fontSize: 12,
    textTransform: 'uppercase',
    letterSpacing: 0.7,
  },
  summaryValue: {
    fontSize: 28,
    fontWeight: '800',
  },
  panel: {
    borderWidth: 1,
    borderRadius: 22,
    padding: 18,
    gap: 14,
  },
  panelTitle: {
    fontSize: 16,
    fontWeight: '700',
  },
  panelCopy: {
    fontSize: 13,
    marginTop: -8,
  },
  mixBar: {
    height: 18,
    borderWidth: 1,
    borderRadius: 999,
    overflow: 'hidden',
    flexDirection: 'row',
  },
  mixSegment: {
    height: '100%',
  },
  mixLegend: {
    gap: 10,
  },
  mixLegendRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  mixLegendLabel: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  mixLegendDot: {
    width: 10,
    height: 10,
    borderRadius: 999,
  },
  histogramRow: {
    alignItems: 'flex-end',
    gap: 10,
    paddingBottom: 4,
  },
  histogramColumn: {
    width: 32,
    gap: 8,
    alignItems: 'center',
  },
  histogramTrack: {
    width: 24,
    height: 170,
    borderRadius: 999,
    borderWidth: 1,
    overflow: 'hidden',
    justifyContent: 'flex-end',
  },
  histogramSegment: {
    width: '100%',
  },
  histogramLabel: {
    fontSize: 11,
    transform: [{ rotate: '-55deg' }],
    width: 54,
    textAlign: 'right',
  },
  frequencyList: {
    gap: 10,
  },
  frequencyCard: {
    borderWidth: 1,
    borderRadius: 18,
    padding: 14,
    gap: 12,
  },
  frequencyHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    gap: 12,
  },
  frequencyTitle: {
    flex: 1,
    fontWeight: '700',
  },
  frequencyDays: {
    fontSize: 12,
  },
  frequencyStats: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    gap: 8,
  },
  frequencyStat: {
    flex: 1,
    gap: 4,
  },
  frequencyStatLabel: {
    fontSize: 12,
  },
  frequencyStatValue: {
    fontSize: 22,
    fontWeight: '800',
  },
  emptyCopy: {
    fontSize: 14,
  },
});
