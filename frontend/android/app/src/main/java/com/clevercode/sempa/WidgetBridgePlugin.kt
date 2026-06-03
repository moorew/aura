package com.clevercode.sempa

import android.content.Context
import androidx.glance.appwidget.updateAll
import com.clevercode.sempa.widget.SempaLargeWidget
import com.clevercode.sempa.widget.SempaMediumWidget
import com.clevercode.sempa.widget.SempaSmallWidget
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

@CapacitorPlugin(name = "WidgetBridge")
class WidgetBridgePlugin : Plugin() {

    @PluginMethod
    fun updateWidgetData(call: PluginCall) {
        val ctx = context ?: run {
            call.reject("No context")
            return
        }

        val prefs = ctx.getSharedPreferences("sempa_widget", Context.MODE_PRIVATE)
        val editor = prefs.edit()

        // Today stats
        call.getInt("todayTotal")?.let { editor.putInt("today_total", it) }
        call.getInt("todayDone")?.let { editor.putInt("today_done", it) }

        // Task list (for medium/large widgets)
        val tasks = call.getArray("tasks")
        if (tasks != null) {
            // Clear old task entries
            for (i in 0..9) {
                editor.remove("task_${i}_title")
                editor.remove("task_${i}_done")
            }
            for (i in 0 until minOf(tasks.length(), 10)) {
                val t = tasks.getJSONObject(i)
                editor.putString("task_${i}_title", t.optString("title", ""))
                editor.putBoolean("task_${i}_done", t.optBoolean("done", false))
            }
            editor.putInt("task_count", minOf(tasks.length(), 10))
        }

        // Week data (for large widget)
        val week = call.getArray("week")
        if (week != null) {
            for (i in 0 until minOf(week.length(), 7)) {
                val d = week.getJSONObject(i)
                val date = d.optString("date", "")
                val count = d.optInt("count", 0)
                if (date.isNotEmpty()) {
                    editor.putInt("week_${date}_count", count)
                }
            }
        }

        editor.apply()

        // Trigger widget refresh
        CoroutineScope(Dispatchers.IO).launch {
            try {
                SempaSmallWidget().updateAll(ctx)
                SempaMediumWidget().updateAll(ctx)
                SempaLargeWidget().updateAll(ctx)
            } catch (_: Exception) {}
        }

        call.resolve()
    }
}
