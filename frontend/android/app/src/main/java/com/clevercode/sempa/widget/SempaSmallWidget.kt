package com.clevercode.sempa.widget

import android.content.Context
import android.content.Intent
import android.net.Uri
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
import java.time.LocalDate
import java.time.format.DateTimeFormatter

class SempaSmallWidget : GlanceAppWidget() {

    override suspend fun provideGlance(context: Context, id: GlanceId) {
        val prefs = context.getSharedPreferences("sempa_widget", Context.MODE_PRIVATE)
        val total = prefs.getInt("today_total", 0)
        val done = prefs.getInt("today_done", 0)

        provideContent {
            SmallWidgetContent(total = total, done = done)
        }
    }
}

@Composable
private fun SmallWidgetContent(total: Int, done: Int) {
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
        Column(
            modifier = GlanceModifier.fillMaxSize(),
            verticalAlignment = Alignment.Vertical.Top,
        ) {
            // Header
            Row(verticalAlignment = Alignment.Vertical.CenterVertically) {
                // Sempa mark
                Text(
                    text = "\u2313",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.primary),
                        fontSize = 20.sp,
                        fontWeight = FontWeight.Bold,
                    ),
                )
                Spacer(modifier = GlanceModifier.width(6.dp))
                Text(
                    text = "Today",
                    style = TextStyle(
                        color = ColorProvider(SempaWidgetTheme.onSurface),
                        fontSize = 13.sp,
                        fontWeight = FontWeight.Medium,
                    ),
                )
            }

            Spacer(modifier = GlanceModifier.height(8.dp))

            // Big count
            Text(
                text = "$done/$total",
                style = TextStyle(
                    color = ColorProvider(SempaWidgetTheme.onSurface),
                    fontSize = 32.sp,
                    fontWeight = FontWeight.Bold,
                ),
            )

            // Subtitle
            Text(
                text = if (done == total && total > 0) "All done!" else "$done done",
                style = TextStyle(
                    color = ColorProvider(SempaWidgetTheme.onSurfaceDim),
                    fontSize = 12.sp,
                ),
            )

            Spacer(modifier = GlanceModifier.defaultWeight())

            // Progress bar
            Box(
                modifier = GlanceModifier
                    .fillMaxWidth()
                    .height(4.dp)
                    .cornerRadius(2.dp)
                    .background(SempaWidgetTheme.outline),
            ) {
                if (pct > 0) {
                    Box(
                        modifier = GlanceModifier
                            .fillMaxHeight()
                            .width((pct * 1.2).dp) // approximate scaling
                            .cornerRadius(2.dp)
                            .background(SempaWidgetTheme.primary),
                    ) {}
                }
            }
        }
    }
}

class SempaSmallWidgetReceiver : GlanceAppWidgetReceiver() {
    override val glanceAppWidget: GlanceAppWidget = SempaSmallWidget()
}
