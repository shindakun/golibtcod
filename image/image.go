// Package image is a faithful Go port of libtcod's image_c.c: a mipmapped
// RGB image with console blitting, including the subcell quadrant
// rendering that doubles a console's effective resolution.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
//
// One deliberate deviation: libtcod loads and saves images through SDL,
// which golibtcod does not depend on. Load/Save here use Go's stdlib
// image/png and image/jpeg decoders instead, which is strictly more
// capable than the C version for PNG and needs no external library. The
// pixel operations (mipmaps, scale, blit, blit2x) are line-by-line
// ports.
package image

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"golibtcod/color"
	"golibtcod/console"
)

type mipmap struct {
	width, height   int
	fwidth, fheight float32
	buf             []color.RGB
	dirty           bool
}

// Image mirrors TCOD_Image: a pixel buffer plus lazily-generated mipmaps.
type Image struct {
	mipmaps     []mipmap
	hasKeyColor bool
	keyColor    color.RGB
}

// mipmapLevels is TCOD_image_get_mipmap_levels.
func mipmapLevels(width, height int) int {
	n := 0
	for width > 0 && height > 0 {
		n++
		width >>= 1
		height >>= 1
	}
	return n
}

// New is TCOD_image_new: a black image with its mipmap chain allocated.
func New(width, height int) *Image {
	if width <= 0 || height <= 0 {
		return nil
	}
	img := &Image{mipmaps: make([]mipmap, mipmapLevels(width, height))}
	img.mipmaps[0].buf = make([]color.RGB, width*height)
	fw, fh := float32(width), float32(height)
	for i := range img.mipmaps {
		img.mipmaps[i].width = width
		img.mipmaps[i].height = height
		img.mipmaps[i].fwidth = fw
		img.mipmaps[i].fheight = fh
		width >>= 1
		height >>= 1
		fw *= 0.5
		fh *= 0.5
	}
	return img
}

// Size is TCOD_image_get_size.
func (img *Image) Size() (w, h int) {
	if img == nil || len(img.mipmaps) == 0 {
		return 0, 0
	}
	return img.mipmaps[0].width, img.mipmaps[0].height
}

func (img *Image) inBounds(x, y int) bool {
	if img == nil || len(img.mipmaps) == 0 {
		return false
	}
	return x >= 0 && y >= 0 && x < img.mipmaps[0].width && y < img.mipmaps[0].height
}

func (img *Image) invalidateMipmaps() {
	for i := 1; i < len(img.mipmaps); i++ {
		img.mipmaps[i].dirty = true
	}
}

// generateMip is TCOD_image_generate_mip: box-filter the level-0 buffer.
func (img *Image) generateMip(mip int) {
	orig := &img.mipmaps[0]
	cur := &img.mipmaps[mip]
	if cur.buf == nil {
		cur.buf = make([]color.RGB, cur.width*cur.height)
	}
	cur.dirty = false
	for x := 0; x < cur.width; x++ {
		for y := 0; y < cur.height; y++ {
			var r, g, b, count int
			for sx := x << mip; sx < (x+1)<<mip; sx++ {
				for sy := y << mip; sy < (y+1)<<mip; sy++ {
					offset := sx + sy*orig.width
					if offset < 0 || offset >= len(orig.buf) {
						continue
					}
					count++
					r += int(orig.buf[offset].R)
					g += int(orig.buf[offset].G)
					b += int(orig.buf[offset].B)
				}
			}
			if count == 0 {
				continue
			}
			cur.buf[x+y*cur.width] = color.RGB{
				R: uint8(r / count), G: uint8(g / count), B: uint8(b / count),
			}
		}
	}
}

// Clear is TCOD_image_clear.
func (img *Image) Clear(c color.RGB) {
	if img == nil {
		return
	}
	for i := range img.mipmaps[0].buf {
		img.mipmaps[0].buf[i] = c
	}
	img.invalidateMipmaps()
}

// Pixel is TCOD_image_get_pixel; out-of-bounds reads return black, as in C.
func (img *Image) Pixel(x, y int) color.RGB {
	if !img.inBounds(x, y) {
		return color.RGB{}
	}
	return img.mipmaps[0].buf[x+y*img.mipmaps[0].width]
}

// PutPixel is TCOD_image_put_pixel.
func (img *Image) PutPixel(x, y int, c color.RGB) {
	if !img.inBounds(x, y) {
		return
	}
	img.mipmaps[0].buf[x+y*img.mipmaps[0].width] = c
	img.invalidateMipmaps()
}

// Alpha is TCOD_image_get_alpha. The C implementation always returns 255
// for non-SDL images; kept for API compatibility.
func (img *Image) Alpha(x, y int) int { return 255 }

// SetKeyColor is TCOD_image_set_key_color: pixels of this color are
// treated as transparent when blitting.
func (img *Image) SetKeyColor(c color.RGB) {
	img.hasKeyColor = true
	img.keyColor = c
}

// IsPixelTransparent is TCOD_image_is_pixel_transparent.
func (img *Image) IsPixelTransparent(x, y int) bool {
	c := img.Pixel(x, y)
	return img.hasKeyColor && c == img.keyColor
}

// MipmapPixel is TCOD_image_get_mipmap_pixel: sample the mipmap level
// appropriate to the requested texel footprint.
func (img *Image) MipmapPixel(x0, y0, x1, y1 float32) color.RGB {
	if img == nil || len(img.mipmaps) == 0 {
		return color.RGB{}
	}
	curSize := 1
	mip := 0
	texelXSize := int(x1 - x0)
	texelYSize := int(y1 - y0)
	texelSize := texelXSize
	if texelYSize > texelXSize {
		texelSize = texelYSize
	}
	for mip < len(img.mipmaps)-1 && curSize < texelSize {
		mip++
		curSize <<= 1
	}
	if mip > 0 {
		mip--
	}
	texelX := int(x0 * float32(img.mipmaps[mip].width) / img.mipmaps[0].fwidth)
	texelY := int(y0 * float32(img.mipmaps[mip].height) / img.mipmaps[0].fheight)
	if img.mipmaps[mip].buf == nil || img.mipmaps[mip].dirty {
		img.generateMip(mip)
	}
	if texelX < 0 || texelY < 0 || texelX >= img.mipmaps[mip].width || texelY >= img.mipmaps[mip].height {
		return color.RGB{}
	}
	return img.mipmaps[mip].buf[texelX+texelY*img.mipmaps[mip].width]
}

/* --------------------------------------------------------- transforms */

// Invert is TCOD_image_invert.
func (img *Image) Invert() {
	if img == nil {
		return
	}
	for i, c := range img.mipmaps[0].buf {
		img.mipmaps[0].buf[i] = color.RGB{R: 255 - c.R, G: 255 - c.G, B: 255 - c.B}
	}
	img.invalidateMipmaps()
}

// HFlip is TCOD_image_hflip.
func (img *Image) HFlip() {
	w, h := img.Size()
	for py := 0; py < h; py++ {
		for px := 0; px < w/2; px++ {
			a := img.Pixel(px, py)
			b := img.Pixel(w-1-px, py)
			img.PutPixel(px, py, b)
			img.PutPixel(w-1-px, py, a)
		}
	}
}

// VFlip is TCOD_image_vflip.
func (img *Image) VFlip() {
	w, h := img.Size()
	for px := 0; px < w; px++ {
		for py := 0; py < h/2; py++ {
			a := img.Pixel(px, py)
			b := img.Pixel(px, h-1-py)
			img.PutPixel(px, py, b)
			img.PutPixel(px, h-1-py, a)
		}
	}
}

// Rotate90 is TCOD_image_rotate90 (quarter turns clockwise).
func (img *Image) Rotate90(rotations int) {
	rotations = rotations % 4
	if rotations < 0 {
		rotations += 4
	}
	for k := 0; k < rotations; k++ {
		w, h := img.Size()
		newImg := New(h, w)
		for px := 0; px < w; px++ {
			for py := 0; py < h; py++ {
				newImg.PutPixel(h-1-py, px, img.Pixel(px, py))
			}
		}
		img.replace(newImg)
	}
}

func (img *Image) replace(other *Image) {
	img.mipmaps = other.mipmaps
}

// Scale is TCOD_image_scale: supersampled when shrinking, nearest
// neighbour when growing.
func (img *Image) Scale(newW, newH int) {
	width, height := img.Size()
	if (newW == width && newH == height) || newW <= 0 || newH <= 0 {
		return
	}
	newImage := New(newW, newH)

	if newW < width && newH < height {
		// scale down with supersampling: fractional edges, centre, corners
		for py := 0; py < newH; py++ {
			y0 := float32(py) * float32(height) / float32(newH)
			y0floor := float32(math.Floor(float64(y0)))
			y0weight := 1.0 - (y0 - y0floor)
			iy0 := int(y0floor)

			y1 := float32(py+1) * float32(height) / float32(newH)
			y1floor := float32(math.Floor(float64(y1 - 0.00001)))
			y1weight := y1 - y1floor
			iy1 := int(y1floor)

			for px := 0; px < newW; px++ {
				x0 := float32(px) * float32(width) / float32(newW)
				x0floor := float32(math.Floor(float64(x0)))
				x0weight := 1.0 - (x0 - x0floor)
				ix0 := int(x0floor)

				x1 := float32(px+1) * float32(width) / float32(newW)
				x1floor := float32(math.Floor(float64(x1 - 0.00001)))
				x1weight := x1 - x1floor
				ix1 := int(x1floor)

				var r, g, b, sumWeight float32
				for srcy := int(y0) + 1; srcy < int(y1); srcy++ {
					colLeft := img.Pixel(ix0, srcy)
					colRight := img.Pixel(ix1, srcy)
					r += float32(colLeft.R)*x0weight + float32(colRight.R)*x1weight
					g += float32(colLeft.G)*x0weight + float32(colRight.G)*x1weight
					b += float32(colLeft.B)*x0weight + float32(colRight.B)*x1weight
					sumWeight += x0weight + x1weight
				}
				for srcx := int(x0) + 1; srcx < int(x1); srcx++ {
					colTop := img.Pixel(srcx, iy0)
					colBottom := img.Pixel(srcx, iy1)
					r += float32(colTop.R)*y0weight + float32(colBottom.R)*y1weight
					g += float32(colTop.G)*y0weight + float32(colBottom.G)*y1weight
					b += float32(colTop.B)*y0weight + float32(colBottom.B)*y1weight
					sumWeight += y0weight + y1weight
				}
				for srcy := int(y0) + 1; srcy < int(y1); srcy++ {
					for srcx := int(x0) + 1; srcx < int(x1); srcx++ {
						s := img.Pixel(srcx, srcy)
						r += float32(s.R)
						g += float32(s.G)
						b += float32(s.B)
						sumWeight += 1.0
					}
				}
				corner := func(cx, cy int, w float32) {
					c := img.Pixel(cx, cy)
					r += float32(c.R) * w
					g += float32(c.G) * w
					b += float32(c.B) * w
					sumWeight += w
				}
				corner(ix0, iy0, x0weight*y0weight)
				corner(ix0, iy1, x0weight*y1weight)
				corner(ix1, iy1, x1weight*y1weight)
				corner(ix1, iy0, x1weight*y0weight)

				if sumWeight <= 0 {
					continue
				}
				inv := 1.0 / sumWeight
				newImage.PutPixel(px, py, color.RGB{
					R: uint8(r*inv + 0.5), G: uint8(g*inv + 0.5), B: uint8(b*inv + 0.5),
				})
			}
		}
	} else {
		// scale up with nearest neighbour
		for py := 0; py < newH; py++ {
			srcy := py * height / newH
			for px := 0; px < newW; px++ {
				srcx := px * width / newW
				newImage.PutPixel(px, py, img.Pixel(srcx, srcy))
			}
		}
	}
	img.replace(newImage)
}

/* ------------------------------------------------------- console blit */

// FromConsole is TCOD_image_from_console: a 2x-resolution image of a
// console's background colors.
func FromConsole(c *console.Console) *Image {
	if c == nil {
		return nil
	}
	img := New(c.W*2, c.H*2)
	RefreshConsole(img, c)
	return img
}

// RefreshConsole is TCOD_image_refresh_console.
func RefreshConsole(img *Image, c *console.Console) {
	if img == nil || c == nil {
		return
	}
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			bg := c.CharBackground(x, y)
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					img.PutPixel(x*2+dx, y*2+dy, bg)
				}
			}
		}
	}
}

// BlitRect is TCOD_image_blit_rect: fit the image into a console rect.
// Pass w or h of -1 to use the image's own dimension.
func (img *Image) BlitRect(c *console.Console, x, y, w, h int, flag console.BkgndFlag) {
	if img == nil || c == nil {
		return
	}
	width, height := img.Size()
	if w == -1 {
		w = width
	}
	if h == -1 {
		h = height
	}
	if w <= 0 || h <= 0 || flag == console.BkgndNone {
		return
	}
	scaleX := float32(w) / float32(width)
	scaleY := float32(h) / float32(height)
	img.Blit(c, float32(x)+float32(w)*0.5, float32(y)+float32(h)*0.5, flag, scaleX, scaleY, 0)
}

// Blit is TCOD_image_blit: draw the image into a console's background
// layer, centred on (x,y), with optional scaling and rotation.
func (img *Image) Blit(c *console.Console, x, y float32, flag console.BkgndFlag, scaleX, scaleY, angle float32) {
	if img == nil || c == nil || scaleX == 0 || scaleY == 0 || flag == console.BkgndNone {
		return
	}
	width, height := img.Size()
	rx := x - float32(width)*0.5
	ry := y - float32(height)*0.5

	if scaleX == 1.0 && scaleY == 1.0 && angle == 0.0 &&
		rx == float32(int(rx)) && ry == float32(int(ry)) {
		// fast path: axis-aligned, unscaled, integer-positioned
		ix := int(x - float32(width)*0.5)
		iy := int(y - float32(height)*0.5)
		minX, minY := maxI(ix, 0), maxI(iy, 0)
		maxX, maxY := minI(ix+width, c.W), minI(iy+height, c.H)
		offsetX, offsetY := 0, 0
		if ix < 0 {
			offsetX = -ix
		}
		if iy < 0 {
			offsetY = -iy
		}
		for cx := minX; cx < maxX; cx++ {
			for cy := minY; cy < maxY; cy++ {
				col := img.Pixel(cx-minX+offsetX, cy-minY+offsetY)
				if !img.hasKeyColor || col != img.keyColor {
					c.SetCharBackground(cx, cy, col, flag)
				}
			}
		}
		return
	}

	iw := float32(width) / 2 * scaleX
	ih := float32(height) / 2 * scaleY
	newXX := float32(math.Cos(float64(angle)))
	newXY := -float32(math.Sin(float64(angle)))
	newYX := newXY
	newYY := -newXX

	x0 := int(x - iw*newXX + ih*newYX)
	y0 := int(y - iw*newXY + ih*newYY)
	x1 := int(x + iw*newXX + ih*newYX)
	y1 := int(y + iw*newXY + ih*newYY)
	x2 := int(x + iw*newXX - ih*newYX)
	y2 := int(y + iw*newXY - ih*newYY)
	x3 := int(x - iw*newXX - ih*newYX)
	y3 := int(y - iw*newXY - ih*newYY)

	rxi := minI(minI(x0, x1), minI(x2, x3))
	ryi := minI(minI(y0, y1), minI(y2, y3))
	rw := maxI(maxI(x0, x1), maxI(x2, x3)) - rxi
	rh := maxI(maxI(y0, y1), maxI(y2, y3)) - ryi

	minX, minY := maxI(rxi, 0), maxI(ryi, 0)
	maxX, maxY := minI(rxi+rw, c.W), minI(ryi+rh, c.H)
	invScaleX := 1.0 / scaleX
	invScaleY := 1.0 / scaleY

	for cx := minX; cx < maxX; cx++ {
		for cy := minY; cy < maxY; cy++ {
			ix := (iw + (float32(cx)-x)*newXX + (float32(cy)-y)*(-newYX)) * invScaleX
			iy := (ih + (float32(cx)-x)*newXY - (float32(cy)-y)*newYY) * invScaleY
			col := img.Pixel(int(ix), int(iy))
			if !img.hasKeyColor || col != img.keyColor {
				if scaleX < 1.0 || scaleY < 1.0 {
					col = img.MipmapPixel(ix, iy, ix+1.0, iy+1.0)
				}
				c.SetCharBackground(cx, cy, col, flag)
			}
		}
	}
}

/* ------------------------------------------- subcell (2x) rendering */

func rgbSquaredDistance(c1, c2 color.RGB) int {
	dr := int(c1.R) - int(c2.R)
	dg := int(c1.G) - int(c2.G)
	db := int(c1.B) - int(c2.B)
	return dr*dr + dg*dg + db*db
}

// quadrantToCodepoint maps a quadrant bitmask to a block glyph. A negative
// codepoint means the fg/bg pair must be swapped. Quadrant bits:
//
//	X 1
//	2 4
var quadrantToCodepoint = [8]int{
	0,
	0x259D,  // upper right
	0x2597,  // lower left
	-0x259A, // upper left + lower right
	0x2596,  // lower right
	0x2590,  // right half
	-0x2580, // upper half
	-0x2598, // upper left
}

// generateQuadrantGraphic is the heart of subcell rendering: reduce four
// pixel colors to one glyph plus a fg/bg pair, merging colors by smallest
// perceptual distance when more than two are present. Adapted in libtcod
// from Jeff Lait's code.
func generateQuadrantGraphic(desired [4]color.RGB) console.Tile {
	quadrantMask := 0
	quadrantIndex := 1
	for ; quadrantIndex < 4; quadrantIndex++ {
		if desired[quadrantIndex] != desired[0] {
			break
		}
	}
	if quadrantIndex == 4 { // solid colour
		c := desired[0]
		rgba := color.RGBA{R: c.R, G: c.G, B: c.B, A: 255}
		return console.Tile{Ch: ' ', Fg: rgba, Bg: rgba}
	}
	palette := [2]color.RGB{desired[0], desired[quadrantIndex]}
	weight := [2]int{quadrantIndex, 1}
	quadrantMask |= 1 << (quadrantIndex - 1)

	for quadrantIndex++; quadrantIndex < 4; quadrantIndex++ {
		switch {
		case desired[quadrantIndex] == palette[0]:
			weight[0]++
		case desired[quadrantIndex] == palette[1]:
			quadrantMask |= 1 << (quadrantIndex - 1)
			weight[1]++
		default:
			// only two colours are representable: merge the closest pair
			dist0q := rgbSquaredDistance(desired[quadrantIndex], palette[0])
			dist1q := rgbSquaredDistance(desired[quadrantIndex], palette[1])
			dist01 := rgbSquaredDistance(palette[0], palette[1])
			if dist0q < dist1q {
				if dist0q <= dist01 {
					palette[0] = color.Lerp(desired[quadrantIndex], palette[0],
						float32(weight[0])/(1.0+float32(weight[0])))
					weight[0]++
				} else {
					palette[0] = color.Lerp(palette[0], palette[1],
						float32(weight[1])/float32(weight[0]+weight[1]))
					weight[0]++
					palette[1] = desired[quadrantIndex]
					quadrantMask = 1 << (quadrantIndex - 1)
				}
			} else {
				if dist1q <= dist01 {
					palette[1] = color.Lerp(desired[quadrantIndex], palette[1],
						float32(weight[1])/(1.0+float32(weight[1])))
					weight[1]++
					quadrantMask |= 1 << (quadrantIndex - 1)
				} else {
					palette[0] = color.Lerp(palette[0], palette[1],
						float32(weight[1])/float32(weight[0]+weight[1]))
					weight[0]++
					palette[1] = desired[quadrantIndex]
					quadrantMask = 1 << (quadrantIndex - 1)
				}
			}
		}
	}

	cp := quadrantToCodepoint[quadrantMask]
	if cp >= 0 {
		return console.Tile{
			Ch: cp,
			Fg: color.RGBA{R: palette[1].R, G: palette[1].G, B: palette[1].B, A: 255},
			Bg: color.RGBA{R: palette[0].R, G: palette[0].G, B: palette[0].B, A: 255},
		}
	}
	return console.Tile{ // negative codepoint: swap fg/bg
		Ch: -cp,
		Fg: color.RGBA{R: palette[0].R, G: palette[0].G, B: palette[0].B, A: 255},
		Bg: color.RGBA{R: palette[1].R, G: palette[1].G, B: palette[1].B, A: 255},
	}
}

// Blit2x is TCOD_image_blit_2x: render the image at twice the console's
// resolution using quadrant block glyphs. Pass srcWidth/srcHeight of -1
// for the whole image.
func (img *Image) Blit2x(c *console.Console, destX, destY, srcX, srcY, srcWidth, srcHeight int) {
	if img == nil || c == nil {
		return
	}
	imgWidth, imgHeight := img.Size()
	if srcWidth == -1 {
		srcWidth = imgWidth
	}
	if srcHeight == -1 {
		srcHeight = imgHeight
	}
	if srcWidth <= 0 || srcHeight <= 0 {
		return
	}
	srcX = maxI(0, srcX)
	srcY = maxI(0, srcY)
	srcWidth = minI(srcWidth, imgWidth-srcX)
	srcHeight = minI(srcHeight, imgHeight-srcY)

	maxX := srcWidth
	if destX+srcWidth/2 > c.W {
		maxX = (c.W - destX) * 2
	}
	maxY := srcHeight
	if destY+srcHeight/2 > c.H {
		maxY = (c.H - destY) * 2
	}
	if destX+maxX/2 < 0 || destY+maxY/2 < 0 || destX >= c.W || destY >= c.H {
		return
	}
	maxX += srcX
	maxY += srcY

	for imgX := srcX; imgX < maxX; imgX += 2 {
		for imgY := srcY; imgY < maxY; imgY += 2 {
			consoleX := destX + (imgX-srcX)/2
			consoleY := destY + (imgY-srcY)/2
			if !c.In(consoleX, consoleY) {
				continue
			}
			consoleBack := c.CharBackground(consoleX, consoleY)
			sample := func(px, py int, ok bool) color.RGB {
				if !ok {
					return consoleBack
				}
				col := img.Pixel(px, py)
				if img.hasKeyColor && col == img.keyColor {
					return consoleBack
				}
				return col
			}
			var grid [4]color.RGB
			grid[0] = sample(imgX, imgY, true)
			grid[1] = sample(imgX+1, imgY, imgX < maxX-1)
			grid[2] = sample(imgX, imgY+1, imgY < maxY-1)
			grid[3] = sample(imgX+1, imgY+1, imgX < maxX-1 && imgY < maxY-1)
			c.Tiles[consoleY*c.W+consoleX] = generateQuadrantGraphic(grid)
		}
	}
}

/* ------------------------------------------------------------- file IO */

// Load reads a PNG or JPEG into an Image.
//
// libtcod does this through SDL_image; golibtcod uses the standard library,
// which needs no external dependency and handles PNG strictly better than
// the C version's built-in loader.
func Load(path string) (*Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var src image.Image
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		src, err = png.Decode(f)
	case ".jpg", ".jpeg":
		src, err = jpeg.Decode(f)
	default:
		src, _, err = image.Decode(f)
	}
	if err != nil {
		return nil, fmt.Errorf("image: decoding %s: %w", path, err)
	}
	return FromGoImage(src), nil
}

// FromGoImage converts any stdlib image.Image into a golibtcod Image.
func FromGoImage(src image.Image) *Image {
	b := src.Bounds()
	img := New(b.Dx(), b.Dy())
	if img == nil {
		return nil
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bb, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			img.PutPixel(x, y, color.RGB{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bb >> 8)})
		}
	}
	return img
}

// ToGoImage converts to a stdlib image for encoding.
func (img *Image) ToGoImage() image.Image {
	w, h := img.Size()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.Pixel(x, y)
			i := out.PixOffset(x, y)
			out.Pix[i] = c.R
			out.Pix[i+1] = c.G
			out.Pix[i+2] = c.B
			out.Pix[i+3] = 255
		}
	}
	return out
}

// Save writes the image as a PNG (TCOD_image_save).
func (img *Image) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img.ToGoImage())
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
