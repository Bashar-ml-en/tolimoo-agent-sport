import { Stack } from 'expo-router';
import { colors } from '../src/theme';

export default function Layout() {
  return (
    <Stack
      screenOptions={{
        headerStyle: { backgroundColor: colors.paper },
        headerTintColor: colors.ink,
        headerShadowVisible: false,
        headerTitleStyle: { fontWeight: '800' },
        contentStyle: { backgroundColor: colors.paper }
      }}
    >
      <Stack.Screen
        name="index"
        options={{
          headerShown: false
        }}
      />
      <Stack.Screen
        name="agents/[id]"
        options={{
          title: 'JOURNALISTE'
        }}
      />
    </Stack>
  );
}