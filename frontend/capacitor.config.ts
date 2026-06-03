import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
  appId: 'com.clevercode.sempa',
  appName: 'sempa',
  webDir: 'build',
  android: {
    backgroundColor: '#181310',
  },
  plugins: {
    SplashScreen: {
      launchAutoHide: true,
      backgroundColor: '#181310',
      androidSplashResourceName: 'splash',
      showSpinner: false,
    },
    PushNotifications: {
      presentationOptions: ['badge', 'sound', 'alert'],
    },
  },
};

export default config;
