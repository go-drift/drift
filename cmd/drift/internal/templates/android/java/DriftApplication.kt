/**
 * DriftApplication registers Drift's channels and the app's plugins once
 * per process, before anything else runs: an Activity, or a service or
 * receiver the system starts without one (a push message, an alarm).
 */
package {{.PackageName}}

import android.app.Application
import com.drift.runner.DriftPlugins

class DriftApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        PlatformChannelManager.init(this)
        DriftPlugins.register(PlatformChannelManager)
    }
}
