package com.clevercode.sempa;

import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.os.Build;
import android.os.Bundle;
import android.os.VibrationEffect;
import android.os.Vibrator;
import android.os.VibratorManager;
import android.webkit.JavascriptInterface;

import com.clevercode.sempa.widget.WidgetRefreshWorker;
import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        registerPlugin(WidgetBridgePlugin.class);
        super.onCreate(savedInstanceState);
        createNotificationChannels();
        WidgetRefreshWorker.Companion.enqueuePeriodicRefresh(this);

        // Expose haptics to WebView
        getBridge().getWebView().addJavascriptInterface(new HapticsInterface(), "SempaHaptics");
    }

    private void createNotificationChannels() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationManager manager = getSystemService(NotificationManager.class);
            if (manager == null) return;

            NotificationChannel reminders = new NotificationChannel(
                "reminders",
                "Task Reminders",
                NotificationManager.IMPORTANCE_HIGH
            );
            reminders.setDescription("Reminders for upcoming and overdue tasks");
            reminders.enableVibration(true);
            manager.createNotificationChannel(reminders);

            NotificationChannel sync = new NotificationChannel(
                "sync",
                "Sync Status",
                NotificationManager.IMPORTANCE_LOW
            );
            sync.setDescription("Integration sync notifications");
            manager.createNotificationChannel(sync);
        }
    }

    /** JavascriptInterface exposed to the WebView for native haptic feedback. */
    private class HapticsInterface {

        @JavascriptInterface
        public void click() {
            vibrate(VibrationEffect.EFFECT_CLICK);
        }

        @JavascriptInterface
        public void tick() {
            vibrate(VibrationEffect.EFFECT_TICK);
        }

        @JavascriptInterface
        public void heavyClick() {
            vibrate(VibrationEffect.EFFECT_HEAVY_CLICK);
        }

        private void vibrate(int effectId) {
            if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) return;

            Vibrator vibrator;
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                VibratorManager vm = getSystemService(VibratorManager.class);
                vibrator = vm != null ? vm.getDefaultVibrator() : null;
            } else {
                vibrator = (Vibrator) getSystemService(VIBRATOR_SERVICE);
            }

            if (vibrator != null && vibrator.hasVibrator()) {
                vibrator.vibrate(VibrationEffect.createPredefined(effectId));
            }
        }
    }
}
