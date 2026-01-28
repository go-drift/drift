//go:build !svgdebug

package graphics

import "unsafe"

func svgDebugTrack(ptr unsafe.Pointer)        {}
func svgDebugUntrack(ptr unsafe.Pointer)      {}
func SVGDebugCheckDestroy(ptr unsafe.Pointer) {}
