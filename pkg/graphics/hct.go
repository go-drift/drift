package graphics

import "math"

// Default CAM16 viewing conditions (sRGB-like display).
// Precomputed from: whitePoint=D65, adaptingLuminance=11.72, backgroundLstar=50,
// surround=2.0, discountingIlluminant=false.
var (
	vcN      float64
	vcAw     float64
	vcNbb    float64
	vcNcb    float64
	vcC      float64
	vcNc     float64
	vcFl     float64
	vcFlRoot float64
	vcZ      float64
	vcRgbD   [3]float64
)

func init() {
	// White point D65 in XYZ.
	const wpX, wpY, wpZ = 95.047, 100.0, 108.883

	// M16 * white point.
	rW := 0.401288*wpX + 0.650173*wpY + -0.051461*wpZ
	gW := -0.250268*wpX + 1.204414*wpY + 0.045854*wpZ
	bW := -0.002079*wpX + 0.048952*wpY + 0.953127*wpZ

	adaptingLuminance := (200.0 / math.Pi) * yFromLstar(50.0) / 100.0

	f := 0.8 + 2.0/10.0 // surround=2.0
	var c float64
	if f >= 0.9 {
		c = lerp64(0.59, 0.69, (f-0.9)*10.0)
	} else {
		c = lerp64(0.525, 0.59, (f-0.8)*10.0)
	}

	d := f * (1.0 - (1.0/3.6)*math.Exp((-adaptingLuminance-42.0)/92.0))
	d = clampFloat64(d, 0, 1)

	vcC = c
	vcNc = f

	vcRgbD = [3]float64{
		d*(100.0/rW) + 1.0 - d,
		d*(100.0/gW) + 1.0 - d,
		d*(100.0/bW) + 1.0 - d,
	}

	k := 1.0 / (5.0*adaptingLuminance + 1.0)
	k4 := k * k * k * k
	k4F := 1.0 - k4
	vcFl = k4*adaptingLuminance + 0.1*k4F*k4F*math.Cbrt(5.0*adaptingLuminance)
	vcFlRoot = math.Pow(vcFl, 0.25)

	vcN = yFromLstar(50.0) / wpY
	vcZ = 1.48 + math.Sqrt(vcN)
	vcNbb = 0.725 / math.Pow(vcN, 0.2)
	vcNcb = vcNbb

	rgbAFactors := [3]float64{
		math.Pow(vcFl*vcRgbD[0]*rW/100.0, 0.42),
		math.Pow(vcFl*vcRgbD[1]*gW/100.0, 0.42),
		math.Pow(vcFl*vcRgbD[2]*bW/100.0, 0.42),
	}
	rgbA := [3]float64{
		400.0 * rgbAFactors[0] / (rgbAFactors[0] + 27.13),
		400.0 * rgbAFactors[1] / (rgbAFactors[1] + 27.13),
		400.0 * rgbAFactors[2] / (rgbAFactors[2] + 27.13),
	}
	vcAw = (2.0*rgbA[0] + rgbA[1] + 0.05*rgbA[2]) * vcNbb
}

// HueChroma extracts the CAM16 hue (degrees, 0-360) and chroma from a [Color]
// under default sRGB viewing conditions.
func HueChroma(c Color) (hue, chroma float64) {
	redL := linearized(c.R())
	greenL := linearized(c.G())
	blueL := linearized(c.B())

	x := 0.41233895*redL + 0.35762064*greenL + 0.18051042*blueL
	y := 0.2126*redL + 0.7152*greenL + 0.0722*blueL
	z := 0.01932141*redL + 0.11916382*greenL + 0.95034478*blueL

	rC := 0.401288*x + 0.650173*y + -0.051461*z
	gC := -0.250268*x + 1.204414*y + 0.045854*z
	bC := -0.002079*x + 0.048952*y + 0.953127*z

	rD := vcRgbD[0] * rC
	gD := vcRgbD[1] * gC
	bD := vcRgbD[2] * bC

	rAF := math.Pow(vcFl*math.Abs(rD)/100.0, 0.42)
	gAF := math.Pow(vcFl*math.Abs(gD)/100.0, 0.42)
	bAF := math.Pow(vcFl*math.Abs(bD)/100.0, 0.42)

	rA := signum(rD) * 400.0 * rAF / (rAF + 27.13)
	gA := signum(gD) * 400.0 * gAF / (gAF + 27.13)
	bA := signum(bD) * 400.0 * bAF / (bAF + 27.13)

	a := (11.0*rA + -12.0*gA + bA) / 11.0
	b := (rA + gA - 2.0*bA) / 9.0

	u := (20.0*rA + 20.0*gA + 21.0*bA) / 20.0
	p2 := (40.0*rA + 20.0*gA + bA) / 20.0

	atan2 := math.Atan2(b, a)
	atanDegrees := atan2 * 180.0 / math.Pi
	hue = sanitizeDegreesDouble(atanDegrees)

	ac := p2 * vcNbb
	j := 100.0 * math.Pow(ac/vcAw, vcC*vcZ)

	huePrime := hue
	if hue < 20.14 {
		huePrime = hue + 360
	}
	eHue := 0.25 * (math.Cos(huePrime*math.Pi/180.0+2.0) + 3.8)
	p1 := (50000.0 / 13.0) * eHue * vcNc * vcNcb
	t := p1 * math.Sqrt(a*a+b*b) / (u + 0.305)
	alpha := math.Pow(t, 0.9) * math.Pow(1.64-math.Pow(0.29, vcN), 0.73)
	chroma = alpha * math.Sqrt(j/100.0)
	return hue, chroma
}

// ColorFromHCT returns the sRGB [Color] closest to the given HCT coordinates
// under default viewing conditions. Tone is clamped to [0, 100].
// If the requested chroma is not achievable, it is reduced to the gamut maximum.
func ColorFromHCT(hue, chroma, tone float64) Color {
	tone = clampFloat64(tone, 0, 100)
	return solveToColor(hue, chroma, tone)
}

// linearized converts an sRGB channel (0-255) to linear RGB (0-100).
func linearized(component uint8) float64 {
	return srgbLinearize(float64(component)/255.0) * 100.0
}

// delinearized converts linear RGB (0-100) to sRGB channel (0-255).
func delinearized(rgbComponent float64) uint8 {
	normalized := rgbComponent / 100.0
	var v float64
	if normalized <= 0.0031308 {
		v = normalized * 12.92
	} else {
		v = 1.055*math.Pow(normalized, 1.0/2.4) - 0.055
	}
	return uint8(clampFloat64(math.Round(v*255.0), 0, 255))
}

// lstarFromArgb computes the L* (CIE LAB lightness) of a Color.
func lstarFromArgb(c Color) float64 {
	redL := linearized(c.R())
	greenL := linearized(c.G())
	blueL := linearized(c.B())
	y := 0.2126*redL + 0.7152*greenL + 0.0722*blueL
	return 116.0*labF(y/100.0) - 16.0
}

// yFromLstar converts L* to Y (relative luminance, 0-100).
func yFromLstar(lstar float64) float64 {
	return 100.0 * labInvf((lstar+16.0)/116.0)
}

func labF(t float64) float64 {
	const e = 216.0 / 24389.0
	const kappa = 24389.0 / 27.0
	if t > e {
		return math.Cbrt(t)
	}
	return (kappa*t + 16) / 116
}

func labInvf(ft float64) float64 {
	const e = 216.0 / 24389.0
	const kappa = 24389.0 / 27.0
	ft3 := ft * ft * ft
	if ft3 > e {
		return ft3
	}
	return (116*ft - 16) / kappa
}

func signum(v float64) float64 {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

func lerp64(a, b, t float64) float64 {
	return (1.0-t)*a + t*b
}

func clampFloat64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sanitizeDegreesDouble(degrees float64) float64 {
	degrees = math.Mod(degrees, 360.0)
	if degrees < 0 {
		degrees += 360.0
	}
	return degrees
}
