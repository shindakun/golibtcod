package color

import "testing"

func TestHSVRoundTrip(t *testing.T) {
	cases := []RGB{
		{255, 0, 0}, {0, 255, 0}, {0, 0, 255},
		{127, 63, 200}, {10, 200, 90}, {255, 255, 255}, {0, 0, 0},
	}
	for _, c := range cases {
		h, s, v := c.GetHSV()
		var back RGB
		back.SetHSV(h, s, v)
		// allow 1/255 rounding per channel
		if diff(c.R, back.R) > 1 || diff(c.G, back.G) > 1 || diff(c.B, back.B) > 1 {
			t.Fatalf("round trip %v -> (%v,%v,%v) -> %v", c, h, s, v, back)
		}
	}
}

func diff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func TestArithmeticSaturates(t *testing.T) {
	if got := Add(RGB{200, 200, 200}, RGB{100, 100, 100}); got != (RGB{255, 255, 255}) {
		t.Fatalf("add = %v", got)
	}
	if got := Subtract(RGB{50, 50, 50}, RGB{100, 100, 100}); got != (RGB{0, 0, 0}) {
		t.Fatalf("subtract = %v", got)
	}
	if got := Multiply(RGB{255, 128, 0}, RGB{128, 128, 128}); got.R != 128 || got.G != 64 {
		t.Fatalf("multiply = %v", got)
	}
	if got := MultiplyScalar(RGB{100, 100, 100}, 10); got != (RGB{255, 255, 255}) {
		t.Fatalf("scalar clamp = %v", got)
	}
}

func TestLerpEndpoints(t *testing.T) {
	a, b := RGB{0, 0, 0}, RGB{255, 255, 255}
	if Lerp(a, b, 0) != a || Lerp(a, b, 1) != b {
		t.Fatal("lerp endpoints wrong")
	}
	if mid := Lerp(a, b, 0.5); mid.R != 127 {
		t.Fatalf("lerp midpoint = %v", mid)
	}
}

func TestHueWrapAround(t *testing.T) {
	var c RGB
	c.SetHSV(-90, 1, 1) // negative hue must wrap to 270
	var want RGB
	want.SetHSV(270, 1, 1)
	if c != want {
		t.Fatalf("hue wrap: %v != %v", c, want)
	}
}

func TestNamedTableSpotChecks(t *testing.T) {
	// values straight from libtcod's color.h
	if DesaturatedRed != (RGB{127, 63, 63}) {
		t.Fatalf("DesaturatedRed = %v", DesaturatedRed)
	}
	if DarkerGreen != (RGB{0, 127, 0}) {
		t.Fatalf("DarkerGreen = %v", DarkerGreen)
	}
	if LightestBlue != (RGB{191, 191, 255}) {
		t.Fatalf("LightestBlue = %v", LightestBlue)
	}
}

func TestAlphaBlend(t *testing.T) {
	dst := RGBA{0, 0, 0, 255}
	AlphaBlend(&dst, RGBA{255, 255, 255, 128})
	if dst.A != 255 || dst.R < 120 || dst.R > 136 {
		t.Fatalf("alpha blend = %v", dst)
	}
}
