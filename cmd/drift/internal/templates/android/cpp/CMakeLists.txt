cmake_minimum_required(VERSION 3.22.1)
project(drift_jni C)

add_library(drift_jni SHARED drift_jni.c)

target_link_libraries(drift_jni log dl android nativewindow EGL GLESv2)
