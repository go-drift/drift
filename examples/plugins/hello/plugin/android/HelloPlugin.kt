/**
 * HelloPlugin.kt
 *
 * The hello plugin's native half on Android. It compiles into the app
 * module under its own package and references only com.drift.runner
 * types, never the app's package, so it builds in every app.
 */
package com.example.hello

import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost

class HelloPlugin : DriftPlugin {
    override fun onRegister(host: DriftPluginHost) {
        host.registerChannel("example/hello") {
            method("greeting") { _ ->
                // Plugin code cannot see the app's R class; look the
                // resource up by name.
                val res = host.context.resources
                val id = res.getIdentifier("hello_greeting", "string", host.context.packageName)
                res.getString(id)
            }
        }
    }
}
