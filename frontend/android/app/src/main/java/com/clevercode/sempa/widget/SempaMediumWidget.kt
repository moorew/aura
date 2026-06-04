package com.clevercode.sempa.widget

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.glance.*
import androidx.glance.action.actionStartActivity
import androidx.glance.action.clickable
import androidx.glance.appwidget.GlanceAppWidget
import androidx.glance.appwidget.GlanceAppWidgetReceiver
import androidx.glance.appwidget.provideContent
import androidx.glance.layout.*
import androidx.glance.text.*
import androidx.glance.appwidget.cornerRadius
import androidx.glance.unit.ColorProvider
import com.clevercode.sempa.MainActivity
import java.time.LocalDate
import java.time.format.DateTimeFormatter

class SempaMediumWidget : GlanceAppWidget() {

    override suspend fun provideGlance(context: Context, id: GlanceId) {
        val prefs = context.getSharedPreferences("sempa_widget", Context.MODE_PRIVATE)
        val total = prefs.getInt("today_total", 0)
        val done = prefs.getInt("today_done", 0)

        // Read up to 4 task titles
        val tasks = mutableListOf<Pair<String, Boolean>>()
        for (i in 0 until 4) {
            val title = prefs.getString("task_${i}_title", null) ?: break
            val isDone = prefs.getBoolean("task_${i}_done", false)
            tasks.add(title to isDone)
        }

        provideContent {
            MediumWidgetContent(total = total, done = done, tasks = tasks)
        }
    }
}

@Composable
private fun MediumWidgetContent(total: Int, done: Int, tasks: List<Pair<String, Boolean>>) {
    val today = LocalDate.now().format(DateTimeFormatter.ofPattern("EEE, MMM d"))
    val pct = if (total > 0) (done.toFloat() / total * 100).toInt() else 0

    Box(
        modifier = GlanceModifier
            .fillMaxSize()
            .cornerRadius(20.dp)
            .background(SempaWidgetTheme.surface)
            .padding(16.dp)
            .clickable(actionStartActivity<MainActivity>()),
    ) {
        Column(modifier = GlanceModifier.fillMaxSize()) {
            // Header row
            Row(
                modifier = GlanceModifier.fillMaxWidth(),
                verticalAlignment = Alignment.Vertical.CenterVertically,
            ) {
                Text(
                    text = "\u2313",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.primary),
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                    ),
                )
                Spacer(modifier = GlanceModifier.width(6.dp))
                Text(
                    text = "Today",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.onSurface),
                        fontSize = 14.sp,
                        fontWeight = FontWeight.Bold,
                    ),
                )
                Spacer(modifier = GlanceModifier.width(6.dp))
                Text(
                    text = today,
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.onSurfaceDim),
                        fontSize = 12.sp,
                    ),
                )
                Spacer(modifier = GlanceModifier.defaultWeight())
                Text(
                    text = "$done/$total",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.primary),
                        fontSize = 13.sp,
                        fontWeight = FontWeight.Medium,
                    ),
                )
            }

            // Progress bar
            Spacer(modifier = GlanceModifier.height(8.dp))
            Box(
                modifier = GlanceModifier
                    .fillMaxWidth()
                    .height(3.dp)
                    .cornerRadius(2.dp)
                    .background(SempaWidgetTheme.outline),
            ) {
                if (pct > 0) {
                    Box(
                        modifier = GlanceModifier
                            .fillMaxHeight()
                            .width((pct * 2.8).dp)
                            .cornerRadius(2.dp)
                            .background(SempaWidgetTheme.primary),
                    ) {}
                }
            }

            Spacer(modifier = GlanceModifier.height(8.dp))

            // Task list
            if (tasks.isEmpty()) {
                Text(
                    text = "No tasks for today",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.onSurfaceDim),
                        fontSize = 13.sp,
                    ),
                )
            } else {
                tasks.forEach { (title, isDone) ->
                    Row(
                        modifier = GlanceModifier
                            .fillMaxWidth()
                            .padding(vertical = 3.dp),
                        verticalAlignment = Alignment.Vertical.CenterVertically,
                    ) {
                        // Checkbox indicator
                        Box(
                            modifier = GlanceModifier
                                .size(14.dp)
                                .cornerRadius(7.dp)
                                .background(
                                    if (isDone) SempaWidgetTheme.green
                                    else SempaWidgetTheme.outline
                                ),
                        ) {}
                        Spacer(modifier = GlanceModifier.width(8.dp))
                        Text(
                            text = title,
                            style = TextStyle(
                                color = ColorProvider(
                                    if (isDone) SempaWidgetTheme.onSurfaceDim
                                    else SempaWidgetTheme.onSurface
                                ),
                                fontSize = 13.sp,
                            ),
                            maxLines = 1,
                        )
                    }
                }
            }

            Spacer(modifier = GlanceModifier.defaultWeight())

            // New task button
            Row(
                modifier = GlanceModifier
                    .fillMaxWidth()
                    .padding(top = 4.dp)
                    .cornerRadius(10.dp)
                    .background(SempaWidgetTheme.accentBg)
                    .padding(horizontal = 12.dp, vertical = 8.dp)
                    .clickable(actionStartActivity<MainActivity>()),
                verticalAlignment = Alignment.Vertical.CenterVertically,
            ) {
                Text(
                    text = "+",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.primary),
                        fontSize = 16.sp,
                        fontWeight = FontWeight.Bold,
                    ),
                )
                Spacer(modifier = GlanceModifier.width(6.dp))
                Text(
                    text = "New task",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.primary),
                        fontSize = 13.sp,
                        fontWeight = FontWeight.Medium,
                    ),
                )
            }
        }
    }
}

class SempaMediumWidgetReceiver : GlanceAppWidgetReceiver() {
    override val glanceAppWidget: GlanceAppWidget = SempaMediumWidget()
}
