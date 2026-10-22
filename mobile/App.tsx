import { StatusBar } from 'expo-status-bar';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, StyleSheet, Text, View } from 'react-native';
import { createApi } from '../shared/src/api';
import { formatMoney } from '../shared/src/types';
import type { Order, Ticket } from '../shared/src/types';

const api = createApi(process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080/graphql');

export default function App() {
  const [tab, setTab] = useState<'approvals' | 'orders'>('approvals');
  return (
    <SafeAreaView style={s.screen}>
      <StatusBar style="auto" />
      <View style={s.tabs}>
        {(['approvals', 'orders'] as const).map((t) => (
          <Pressable key={t} onPress={() => setTab(t)} style={[s.tab, tab === t && s.tabOn]}>
            <Text style={tab === t ? s.tabTextOn : s.tabText}>{t === 'approvals' ? 'Approvals' : 'Orders'}</Text>
          </Pressable>
        ))}
      </View>
      {tab === 'approvals' ? <Approvals /> : <Orders />}
    </SafeAreaView>
  );
}

function Approvals() {
  const [tickets, setTickets] = useState<Ticket[] | null>(null);
  const [error, setError] = useState('');
  const load = useCallback(() => {
    api.tickets().then((t) => setTickets(t.filter((x) => x.drafts.some((d) => d.status === 'pending')))).catch((e: Error) => setError(e.message));
  }, []);
  useEffect(load, [load]);

  async function review(ticket: Ticket, approve: boolean) {
    const draft = ticket.drafts.find((d) => d.status === 'pending');
    if (!draft) return;
    try { await api.reviewDraft(draft.id, approve); load(); } catch (e) { setError((e as Error).message); }
  }

  if (error) return <Text style={s.error}>{error}</Text>;
  if (!tickets) return <ActivityIndicator style={{ marginTop: 40 }} />;
  return (
    <FlatList
      data={tickets}
      keyExtractor={(t) => t.id}
      contentContainerStyle={{ padding: 16, gap: 12 }}
      onRefresh={load}
      refreshing={false}
      ListEmptyComponent={<Text style={s.muted}>No drafts waiting. New tickets appear here with a drafted reply.</Text>}
      renderItem={({ item }) => {
        const draft = item.drafts.find((d) => d.status === 'pending')!;
        return (
          <View style={s.card}>
            <Text style={s.title}>{item.subject}</Text>
            <Text style={s.muted}>{item.customer?.name} · {item.category}</Text>
            <Text style={s.body}>{draft.body}</Text>
            <View style={s.actions}>
              <Pressable style={s.primary} onPress={() => review(item, true)}><Text style={s.primaryText}>Approve and send</Text></Pressable>
              <Pressable style={s.secondary} onPress={() => review(item, false)}><Text style={s.secondaryText}>Reject</Text></Pressable>
            </View>
          </View>
        );
      }}
    />
  );
}

function Orders() {
  const [orders, setOrders] = useState<Order[] | null>(null);
  const [error, setError] = useState('');
  useEffect(() => { api.orders().then(setOrders).catch((e: Error) => setError(e.message)); }, []);
  if (error) return <Text style={s.error}>{error}</Text>;
  if (!orders) return <ActivityIndicator style={{ marginTop: 40 }} />;
  return (
    <FlatList
      data={orders}
      keyExtractor={(o) => o.id}
      contentContainerStyle={{ padding: 16, gap: 10 }}
      renderItem={({ item }) => (
        <View style={s.card}>
          <Text style={s.title}>Order #{item.id} · {formatMoney(item.totalCents)}</Text>
          <Text style={s.muted}>{item.customer?.name} · {item.status}</Text>
        </View>
      )}
    />
  );
}

const ACCENT = '#0f6b5c';
const s = StyleSheet.create({
  screen: { flex: 1, backgroundColor: '#f5f6f3' },
  tabs: { flexDirection: 'row', padding: 12, gap: 8 },
  tab: { paddingVertical: 8, paddingHorizontal: 14, borderRadius: 999, borderWidth: 1, borderColor: ACCENT },
  tabOn: { backgroundColor: ACCENT },
  tabText: { color: ACCENT }, tabTextOn: { color: '#fff' },
  card: { backgroundColor: '#fff', borderRadius: 10, padding: 14, borderWidth: 1, borderColor: '#dde2dd' },
  title: { fontSize: 16, fontWeight: '600', color: '#1d2623' },
  muted: { color: '#5d6b66', marginTop: 4 },
  body: { marginTop: 10, color: '#1d2623', lineHeight: 20 },
  actions: { flexDirection: 'row', gap: 8, marginTop: 14 },
  primary: { backgroundColor: ACCENT, paddingVertical: 10, paddingHorizontal: 14, borderRadius: 8 },
  primaryText: { color: '#fff', fontWeight: '600' },
  secondary: { borderColor: ACCENT, borderWidth: 1, paddingVertical: 10, paddingHorizontal: 14, borderRadius: 8 },
  secondaryText: { color: ACCENT, fontWeight: '600' },
  error: { color: '#a23030', padding: 16 },
});
