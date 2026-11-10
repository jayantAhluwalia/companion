// Everything the app needs to receive a push from the API.
// Remote push needs a physical device and a development build (not Expo Go).
import Constants from 'expo-constants';
import * as Device from 'expo-device';
import * as Notifications from 'expo-notifications';
import { Platform } from 'react-native';

export const NUDGE_CATEGORY = 'nudge';

// Show notifications even while the app is in the foreground, so the spike
// is visible without backgrounding the app first.
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowBanner: true,
    shouldShowList: true,
    shouldPlaySound: true,
    shouldSetBadge: false,
  }),
});

// Action buttons for a nudge. All open the app for now (plan § 4): background
// handling is the least reliable part of the pipeline and can come later.
export async function registerNudgeCategory(): Promise<void> {
  if (Platform.OS === 'web') return;
  await Notifications.setNotificationCategoryAsync(NUDGE_CATEGORY, [
    { identifier: 'next_steps', buttonTitle: 'Show next steps', options: { opensAppToForeground: true } },
    { identifier: 'do_it', buttonTitle: "Let's do it", options: { opensAppToForeground: true } },
    { identifier: 'later', buttonTitle: 'Later', options: { opensAppToForeground: true } },
    { identifier: 'not_interested', buttonTitle: 'Not interested', options: { isDestructive: true, opensAppToForeground: true } },
  ]);
}

function projectId(): string | undefined {
  return (
    Constants.easConfig?.projectId ??
    (Constants.expoConfig?.extra as { eas?: { projectId?: string } } | undefined)?.eas?.projectId
  );
}

/** Ask for permission and return the Expo push token, or throw with a reason. */
export async function getPushToken(): Promise<string> {
  if (!Device.isDevice) throw new Error('Remote push needs a physical device, not a simulator.');

  const existing = await Notifications.getPermissionsAsync();
  let status = existing.status;
  if (status !== 'granted') {
    status = (await Notifications.requestPermissionsAsync()).status;
  }
  if (status !== 'granted') throw new Error('Notification permission was not granted.');

  const id = projectId();
  if (!id) throw new Error('No EAS projectId in app config. Run `npx eas-cli@latest init` and rebuild.');

  const token = await Notifications.getExpoPushTokenAsync({ projectId: id });
  return token.data;
}
