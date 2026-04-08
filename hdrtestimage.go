package main

// Create a test image for validating HDR image processing pipelines.  This is intended to be in the Rec.2100
// PQ colorspace, and is specifically aimed for what Photoshop calls "Rec.2100 PQ W203".  Unfortunately, I
// can't find a good way to write 16-bit images from Go in a format that includes the ability to add color
// tags, so this will need to be manually applied.
//
// This profile seems more or less identical to "Hasselblad Rec. ITU-=R BT.2100 PQ", which is the colorspace
// that Hasselblad's "Phocus" raw converter uses when exporting HDR images from a Hasselblad X2D II camera.

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/tiff"

	"github.com/alltom/oklab"
)

var (
	imageSize     = 1024
	grayBandWidth = 64
	graySideBands = 16
	stepsPerStop  = 64

	// The background should be 203 nits, which is (I *think*) SDR white.  Or would be if I could get this
	// to apply color tags to the generates TIFF.  Note that converting the generated image to sRGB in
	// Photoshop doesn't *quite* get the background to match pure white, so this may be a misunderstanding
	// on my part, although it's not really major.
	bg = pqColor(203)

	// PQ constants
	m1 = 2610.0 / 16384.0
	m2 = 2523.0 / 4096.0 * 128
	c1 = 3424.0 / 4096.0
	c2 = 2413.0 / 4096.0 * 32
	c3 = 2392.0 / 4096.0 * 32
)

// This is roughly the PQ EOTF, as specified in ST.2084 and/or Rec 2100.  It maps nits of brightness (cd/m^2)
// into a floating point fraction of the peak brightness.  For 16-bit color, you'd want to multiply times
// 2^16.
func pq(nits float64) float64 {
	v := math.Pow(
		(c1+c2*math.Pow(nits/10000, m1))/
			(c3*math.Pow(nits/10000, m1)+1), m2)
	return min(1, max(v, 0))
}

func pq16(nits float64) uint16 {
	return uint16(pq(nits)*65535.0 + 0.5)
}

func pqColor(nits float64) color.Color {
	level := pq16(nits)
	return image.NewUniform(color.RGBA64{level, level, level, color.Opaque.A})
}

func main() {
	fmt.Printf("Starting\n")

	// Create image
	i := image.NewRGBA64(image.Rect(0, 0, imageSize, imageSize))

	// Clear image to whatever we're using as a background color.
	bgColor := image.NewUniform(bg)
	draw.Draw(i, i.Bounds(), bgColor, image.Point{0, 0}, draw.Src)

	// Draw gray bands
	for y := range imageSize {
		nits := 10000 * math.Pow(0.5, float64(y)/float64(stepsPerStop))
		bandColor := pqColor(nits)
		bandColorPlus := pqColor(2 * nits)
		bandColorMinus := pqColor(nits / 2)

		// Draw the left side, plus 16-pixel 1-stop offsets.
		drawGrayBand(i, y, 0, grayBandWidth, bandColor)
		drawGrayBand(i, y, 0, graySideBands, bandColorPlus)
		drawGrayBand(i, y, grayBandWidth-graySideBands, grayBandWidth, bandColorMinus)

		// Draw the right side, inverted, plus 16-pixel 1-stop offsets.
		ry := imageSize - y
		drawGrayBand(i, ry, imageSize-grayBandWidth, imageSize, bandColor)
		drawGrayBand(i, ry, imageSize-graySideBands, imageSize, bandColorPlus)
		drawGrayBand(i, ry, imageSize-grayBandWidth, imageSize-grayBandWidth+graySideBands, bandColorMinus)
	}

	for yy := range imageSize / stepsPerStop {
		y := yy * stepsPerStop
		level := uint16(0)
		if yy > 6 {
			level = 65535
		}
		bandColor := image.NewUniform(color.RGBA64{level, level, level, color.Opaque.A})
		draw.Draw(i, image.Rect(0, y, grayBandWidth+16, y+1), bandColor, image.Point{0, y}, draw.Src)
		drawLabel(i, 30, y-4, fmt.Sprintf("%2d", yy), bandColor)

		ry := imageSize - y
		draw.Draw(i, image.Rect(imageSize-grayBandWidth-16, ry, imageSize, ry+1), bandColor, image.Point{imageSize - grayBandWidth - 16, ry}, draw.Src)
		drawLabel(i, imageSize-36, ry-4, fmt.Sprintf("%2d", yy), bandColor)
	}

	rings := 13
	for ring := range rings + 1 {
		nits := 10000 / math.Pow(2, float64(rings-ring))
		level := pq(nits)
		pixels := (imageSize - 3*grayBandWidth) / 2
		pixelsPerRing := float64(pixels) / float64(rings)
		outer := pixelsPerRing * (float64(ring) + 0.9)
		inner := pixelsPerRing * (float64(ring) + 0.1)

		drawOKLCH(i, inner, outer, level, 0.1)

		labelLevel := uint16(0)
		if ring < 7 {
			labelLevel = 65535
		}
		labelColor := image.NewUniform(color.RGBA64{labelLevel, labelLevel, labelLevel, color.Opaque.A})
		offset := 4
		if ring < 10 {
			offset = 9 // center
		}
		drawLabel(i, imageSize/2-int(outer)+offset, imageSize/2+6, fmt.Sprintf("%d", ring), labelColor)
	}

	// Prepare to write
	f, err := os.Create("hdrtestimage.tif")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = tiff.Encode(f, i, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Done.\n")

}

func drawGrayBand(i draw.Image, y, left, right int, bandColor color.Color) {
	for x := left; x < right; x++ {
		i.Set(x, y, bandColor)
	}

}

func drawOKLCH(i draw.Image, innerRadius, outerRadius float64, l, c float64) {
	fmt.Printf("Drawing ring from %f to %f at oklch(%f,%f,x)\n", innerRadius, outerRadius, l, c)
	for x := range imageSize {
		xOff := float64(imageSize/2 - x)
		for y := range imageSize {
			yOff := float64(imageSize/2 - y)
			dist := math.Sqrt(xOff*xOff + yOff*yOff)

			if dist > outerRadius || dist < innerRadius {
				continue
			}

			angle := math.Atan2(xOff, yOff)
			h := angle

			lch := oklab.Oklch{l, c, h}
			i.Set(x, y, lch)
		}
	}
}

func drawLabel(i draw.Image, x, y int, label string, col color.Color) {
	point := fixed.Point26_6{fixed.I(x), fixed.I(y)}
	d := &font.Drawer{
		Dst:  i,
		Src:  image.NewUniform(col),
		Face: inconsolata.Regular8x16,
		Dot:  point,
	}
	d.DrawString(label)
}
