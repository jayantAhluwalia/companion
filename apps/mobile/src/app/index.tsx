// Phase 0 spike screen. Proves the pipeline end to end:
//   settings -> register device -> test push -> interpret + push.
// This screen is throwaway; the real capture screen replaces it in Phase 1.
import { useEffect, useState } from 'react';
import { Button, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import * as Notifications from 'expo-notifications';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { api, type Interpretation } from '@/lib/api';
import { getPushToken, registerNudgeCategory } from '@/lib/notifications';
import { loadSettings, saveSettings } from '@/lib/settings';

export default function SpikeScreen() {
  const [apiUrl, setApiUrl] = useState('');
  const [apiToken, setApiToken] = useState('');
  const [thought, setThought] = useState('I should probably sell my old LS2 helmet. It is just sitting around.');
  const [pushToken, setPushToken] = useState<string | null>(null);
  const [log, setLog] = useState<string[]>([]);
  const [result, setResult] = useState<Interpretation | null>(null);
  const [busy, setBusy] = useState(false);

  const say = (line: string) => setLog((l) => [new Date().toLocaleTimeString() + '  ' + line, ...l].slice(0, 30));

  useEffect(() => {
    loadSettings().then((s) => {
      setApiUrl(s.apiUrl);
      setApiToken(s.apiToken);
    });
    registerNudgeCategory().catch((e) => say('category error: ' + String(e)));
    const sub = Notifications.addNotificationResponseReceivedListener((r) => {
      say(`notification response: action=${r.actionIdentifier} data=${JSON.stringify(r.notification.request.content.data)}`);
    });
    return () => sub.remove();
  }, []);

  const run = async (label: string, fn: () => Promise<string>) => {
    setBusy(true);
    try {
      say(label + ': ' + (await fn()));
    } catch (e) {
      say(label + ' FAILED: ' + (e instanceof Error ? e.message : String(e)));
    } finally {
      setBusy(false);
    }
  };

  return (
    <SafeAreaView style={styles.safe}>
      <ScrollView contentContainerStyle={styles.container} keyboardShouldPersistTaps="handled">
        <ThemedText type="title">Phase 0 spike</ThemedText>

        <ThemedText type="subtitle">1. Server</ThemedText>
        <TextInput style={styles.input} placeholder="https://companion-api.fly.dev" autoCapitalize="none" autoCorrect={false} value={apiUrl} onChangeText={setApiUrl} />
        <TextInput style={styles.input} placeholder="API token" autoCapitalize="none" autoCorrect={false} secureTextEntry value={apiToken} onChangeText={setApiToken} />
        <View style={styles.row}>
          <Button title="Save" disabled={busy} onPress={() => run('save', async () => { await saveSettings({ apiUrl, apiToken }); return 'ok'; })} />
          <Button title="Health" disabled={busy} onPress={() => run('health', async () => JSON.stringify(await api.health()))} />
        </View>

        <ThemedText type="subtitle">2. Device</ThemedText>
        <Button
          title={pushToken ? 'Re-register device' : 'Register device for push'}
          disabled={busy}
          onPress={() =>
            run('register', async () => {
              const t = await getPushToken();
              setPushToken(t);
              await api.registerDevice(t);
              return t;
            })
          }
        />

        <ThemedText type="subtitle">3. Push</ThemedText>
        <Button title="Send test push" disabled={busy || !pushToken} onPress={() => run('push', async () => JSON.stringify((await api.pushTest('Companion', 'Push works.')).tickets))} />

        <ThemedText type="subtitle">4. Thought → interpretation → push</ThemedText>
        <TextInput style={[styles.input, styles.multiline]} multiline value={thought} onChangeText={setThought} />
        <Button
          title="Run spike"
          disabled={busy}
          onPress={() =>
            run('spike', async () => {
              const r = await api.spike(thought);
              setResult(r.interpretation);
              return `${r.call.InputTokens}+${r.call.OutputTokens} tokens, ${Math.round(r.call.Latency / 1e6)} ms, tickets=${JSON.stringify(r.tickets)}`;
            })
          }
        />

        {result && (
          <ThemedView style={styles.card}>
            <ThemedText type="subtitle">{result.title}</ThemedText>
            <ThemedText type="small">{result.type} · confidence {result.confidence.toFixed(2)}</ThemedText>
            <ThemedText>{result.description}</ThemedText>
            {result.next_steps.map((s, i) => (
              <ThemedText key={i}>• {s.label} ({s.estimated_minutes} min)</ThemedText>
            ))}
            {result.clarification_needed && <ThemedText type="small">Question: {result.clarification_question}</ThemedText>}
            {result.assumptions.length > 0 && <ThemedText type="small">Assumptions: {result.assumptions.join('; ')}</ThemedText>}
          </ThemedView>
        )}

        <ThemedText type="subtitle">Log</ThemedText>
        {log.map((l, i) => (
          <ThemedText key={i} type="small" selectable>
            {l}
          </ThemedText>
        ))}
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1 },
  container: { padding: 16, gap: 10, paddingBottom: 120 },
  input: { borderWidth: StyleSheet.hairlineWidth, borderColor: '#888', borderRadius: 8, padding: 10, fontSize: 15, color: '#999' },
  multiline: { minHeight: 80, textAlignVertical: 'top' },
  row: { flexDirection: 'row', gap: 12, justifyContent: 'flex-start' },
  card: { borderRadius: 10, padding: 12, gap: 4, borderWidth: StyleSheet.hairlineWidth, borderColor: '#888' },
});
