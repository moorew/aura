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
import com.clevercode.sempa.MainActivity
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import java.time.temporal.TemporalAdjusters

class SempaLargeWidget : GlanceAppWidget() {

    override suspend fun provideGlance(context: Context, id: GlanceId) {
        val prefs = context.getSharedPreferences("sempa_widget", Context.MODE_PRIVATE)
        val total = prefs.getInt("today_total", 0)
        val done = prefs.getInt("today_done", 0)

        // Week day counts
        val weekCounts = mutableListOf<Pair<String, Int>>()
        val today = LocalDate.now()
        val monday = today.with(TemporalAdjusters.previousOrSame(DayOfWeek.MONDAY))
        val dayLabels = listOf("M", "T", "W", "T", "F", "S", "S")
        for (i in 0 until 7) {
            val d = monday.plusDays(i.toLong())
            val key = d.format(DateTimeFormatter.ISO_LOCAL_DATE)
            val count = prefs.getInt("week_${key}_count", 0)
            weekCounts.add(dayLabels[i] to count)
        }

        // Today's tasks (up to 6)
        val tasks = mutableListOf<Pair<String, Boolean>>()
        for (i in 0 until 6) {
            val title = prefs.getString("task_${i}_title", null) ?: break
            val isDone = prefs.getBoolean("task_${i}_done", false)
            tasks.add(title to isDone)
        }

        provideContent {
            LargeWidgetContent(
                total = total, done = done,
                weekCounts = weekCounts,
                tasks = tasks,
                todayIdx = (today.dayOfWeek.value - 1), // 0=Mon
            )
        }
    }
}

@Composable
private fun LargeWidgetContent(
    total: Int, done: Int,
    weekCounts: List<Pair<String, Int>>,
    tasks: List<Pair<String, Boolean>>,
    todayIdx: Int,
) {
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
            // Header
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
                    text = "This week",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.onSurface),
                        fontSize = 14.sp,
                        fontWeight = FontWeight.Bold,
                    ),
                )
                Spacer(modifier = GlanceModifier.defaultWeight())
                Text(
                    text = today,
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.onSurfaceDim),
                        fontSize = 12.sp,
                    ),
                )
            }

            Spacer(modifier = GlanceModifier.height(10.dp))

            // Week grid
            Row(
                modifier = GlanceModifier
                    .fillMaxWidth()
                    .cornerRadius(12.dp)
                    .background(SempaWidgetTheme.surfaceVariant)
                    .padding(horizontal = 8.dp, vertical = 10.dp),
            ) {
                weekCounts.forEachIndexed { idx, (label, count) ->
                    Column(
                        modifier = GlanceModifier.defaultWeight(),
                        horizontalAlignment = Alignment.Horizontal.CenterHorizontally,
                    ) {
                        Text(
                            text = label,
                            style = TextStyle(
                                color = ColorProvider(
                                    if (idx == todayIdx) SempaWidgetTheme.primary
                                    else SempaWidgetTheme.onSurfaceDim
                                ),
                                fontSize = 10.sp,
                                fontWeight = if (idx == todayIdx) FontWeight.Bold else FontWeight.Normal,
                            ),
                        )
                        Spacer(modifier = GlanceModifier.height(2.dp))
                        Box(
                            modifier = GlanceModifier
                                .size(24.dp)
                                .cornerRadius(12.dp)
                                .background(
                                    if (idx == todayIdx) SempaWidgetTheme.primary
                                    else SempaWidgetTheme.surface
                                ),
                            contentAlignment = Alignment.Center,
                        ) {
                            Text(
                                text = "$count",
                                style = TextStyle(
                                    color = ColorProvider(
                                        if (idx == todayIdx) SempaWidgetTheme.onPrimary
                                        else SempaWidgetTheme.onSurface
                                    ),
                                    fontSize = 11.sp,
                                    fontWeight = FontWeight.Medium,
                                ),
                            )
                        }
                    }
                }
            }

            Spacer(modifier = GlanceModifier.height(10.dp))

            // Progress
            Row(
                modifier = GlanceModifier.fillMaxWidth(),
                verticalAlignment = Alignment.Vertical.CenterVertically,
            ) {
                Text(
                    text = "Today",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.onSurface),
                        fontSize = 13.sp,
                        fontWeight = FontWeight.Medium,
                    ),
                )
                Spacer(modifier = GlanceModifier.defaultWeight())
                Text(
                    text = "$done/$total done",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.onSurfaceDim),
                        fontSize = 11.sp,
                    ),
                )
            }

            Spacer(modifier = GlanceModifier.height(4.dp))

            // Progress bar
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
                Box(
                    modifier = GlanceModifier.fillMaxWidth().defaultWeight(),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        text = "No tasks for today",
                        style = TextStyle(
                            color = ColorProvider(SempaWidgetTheme.onSurfaceDim),
                            fontSize = 13.sp,
                        ),
                    )
                }
            } else {
                Column(modifier = GlanceModifier.defaultWeight()) {
                    tasks.forEach { (title, isDone) ->
                        Row(
                            modifier = GlanceModifier
                                .fillMaxWidth()
                                .padding(vertical = 2.dp),
                            verticalAlignment = Alignment.Vertical.CenterVertically,
                        ) {
                            Box(
                                modifier = GlanceModifier
                                    .size(12.dp)
                                    .cornerRadius(6.dp)
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
            }
        }
    }
}

class SempaLargeWidgetReceiver : GlanceAppWidgetReceiver() {
    override val glanceAppWidget: GlanceAppWidget = SempaLargeWidget()
}
