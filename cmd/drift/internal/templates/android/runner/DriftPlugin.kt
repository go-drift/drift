/**
 * DriftPlugin.kt
 *
 * The native half of a Drift plugin on Android. The generated
 * DriftPluginRegistrant creates one instance of each configured plugin per
 * process, in drift.yaml order, and DriftPlugins drives it on the main
 * thread:
 *
 *   1. onRegister once per process, before any Activity is created.
 *      Register channels and event observers here; they live as long as
 *      the process.
 *   2. onPreActivityCreate for every MainActivity creation (including
 *      configuration changes), before super.onCreate.
 *   3. onAttach once the Activity's views exist, and onDetach when the
 *      Activity is destroyed. Activity-bound state (views, listeners) lives
 *      between the two; an Activity recreation detaches and re-attaches.
 *
 * Every method except onRegister has a default no-op implementation.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import android.app.Activity

interface DriftPlugin {
    fun onRegister(host: DriftPluginHost)

    fun onPreActivityCreate(activity: Activity) {}

    fun onAttach(binding: DriftActivityBinding) {}

    fun onDetach() {}
}
