/**
 * DriftPlugin.kt
 *
 * The native half of a Drift plugin on Android. The generated
 * DriftPluginRegistrant creates one instance of each configured plugin per
 * process, in drift.yaml order, and DriftPlugins drives it on the main
 * thread:
 *
 *   1. onRegister once per process, from Application.onCreate, before any
 *      Activity, service or receiver runs (the process may start for a
 *      push message or an alarm, with no Activity). Register channels,
 *      permissions and event observers here; they live as long as the
 *      process.
 *   2. onAttach once an Activity's views exist, and onDetach when that
 *      Activity is destroyed: a pair per Activity, each with its own
 *      binding. A configuration change Drift does not handle itself (dark
 *      mode, locale) destroys the Activity and creates another, and the app
 *      reopened after its Activity finished gets another too. Nothing a
 *      plugin added to the old Activity (overlays, listeners) carries over;
 *      add it again in onAttach. The process, its channels, Go's state and
 *      the platform views (which Drift moves to the new Activity) do.
 *
 * Every method except onRegister has a default no-op implementation.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

interface DriftPlugin {
    fun onRegister(host: DriftPluginHost)

    fun onAttach(binding: DriftActivityBinding) {}

    fun onDetach() {}
}
