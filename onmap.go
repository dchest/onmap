// Package onmap puts pins into a world map image.
package onmap

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sort"

	"github.com/fogleman/gg"
)

//go:embed pin.png
var pinData []byte

//go:embed pin-shadow.png
var pinShadowData []byte

//go:embed mercator.jpg
var mapData []byte

var (
	mapImg    image.Image
	pin       image.Image
	pinShadow image.Image
	pinParts  []image.Image

	mapWidth  int
	mapHeight int
)

func init() {
	pin = decodeImage(pinData)
	pinShadow = decodeImage(pinShadowData)
	pinParts = []image.Image{pinShadow, pin}
	mapImg = decodeImage(mapData)
	mapWidth = mapImg.Bounds().Max.X
	mapHeight = mapImg.Bounds().Max.Y
	StandardCrop = &CropOption{
		Bound:         100,
		MinWidth:      mapWidth / 3,
		MinHeight:     mapHeight / 3,
		PreserveRatio: true,
	}
}

// StandardCrop is the standard crop defined as:
//
// Bound: 100
// MinWidth: mapWidth/3
// MinHeight: mapHeight/3
// PreserveRatio: true
//
var StandardCrop *CropOption

// Coord describes decimal coordinates.
type Coord struct {
	// Latitude
	Lat float64

	// Longitude
	Long float64
}

func (c Coord) latRad() float64 {
	return c.Lat * math.Pi / 180
}

func (c Coord) mercN() float64 {
	return math.Log(math.Tan((math.Pi / 4) + (c.latRad() / 2)))
}

func (c Coord) XY(mapWidth, mapHeight float64) (x int, y int) {
	fx := (c.Long + 180) * (mapWidth / 360)
	fy := (mapHeight / 2) - (mapWidth * c.mercN() / (2 * math.Pi))
	return int(math.Round(fx)), int(math.Round(fy))
}

func (c Coord) SortY() float64 {
	return -(c.mercN() / (2 * math.Pi))
}

func (c Coord) SortX() float64 {
	return c.Long + 180
}

type coordSlice []Coord

func (c coordSlice) Len() int {
	return len(c)
}

func (c coordSlice) Less(i, j int) bool {
	if c[i].SortY() < c[j].SortY() {
		return true
	}
	if c[i].SortX() < c[j].SortX() {
		return true
	}
	return false
}

func (c coordSlice) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func decodeImage(data []byte) image.Image {
	m, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err.Error())
	}
	return m
}

type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

// CropOptions defines options for cropping the map image.
type CropOption struct {
	// Bound is a minimum distance from the pin to the image boundary.
	Bound int

	// MinWidth is a minimum width of image.
	MinWidth int

	// MinHeight is a minimum height of image.
	MinHeight int

	// If PreserveRatio is true, the image preserves the ratio between
	// MinWidth and MinHeight.
	//
	// MinHeight must be less than MinWidth for this to work correctly.
	PreserveRatio bool
}

// MapPins returns an image with the given coordinates marked as pins on the given world map.
// If crop is nil, doesn't crop the image.
//
// World map must be in Mercator projection.
//
// Pin parts are arbitrary pin images, usually  a shadow of the pin and the pin itself.
// Pin parts are drawn on top of each other from the bottom of the map to the top
// by first drawing pinParts[n], then pinParts[n+1], etc.
// The coordinate point is at the bottom center of each pin part image.
func MapPins(worldMap image.Image, pinParts []image.Image, coords []Coord, crop *CropOption) image.Image {
	// Copy and sort coordinates by longitude so that
	// lower pins are drawn on top of upper pins.
	cs := make(coordSlice, len(coords))
	copy(cs, coords)
	sort.Sort(cs)

	// Draw map.
	dc := gg.NewContext(mapImg.Bounds().Max.X, mapImg.Bounds().Max.Y)
	dc.DrawImage(worldMap, 0, 0)

	mw := float64(mapWidth)
	mh := float64(mapHeight)

	// Draw pin parts.
	// Shouldn't draw each part
	for _, pin := range pinParts {
		for _, c := range cs {
			x, y := c.XY(mw, mh)
			dc.DrawImageAnchored(pin, x, y, 0.5, 1)
		}
	}

	// Calculate min&max values.
	maxX := 0
	maxY := 0
	minX := mapWidth
	minY := mapHeight
	for _, c := range cs {
		x, y := c.XY(mw, mh)
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	m := dc.Image()
	if crop == nil {
		return m
	}

	// Calculate bounds.
	minX -= crop.Bound
	if minX < 0 {
		minX = 0
	}
	minY -= crop.Bound
	if minY < 0 {
		minY = 0
	}
	maxX += crop.Bound
	if maxX > mapWidth {
		maxX = mapWidth
	}
	maxY += crop.Bound
	if maxY > mapHeight {
		maxX = mapHeight
	}

	w := maxX - minX
	if w < crop.MinWidth {
		minX -= (crop.MinWidth - w) / 2
		add := 0
		if minX < 0 {
			add = -minX
			minX = 0
		}
		maxX += (crop.MinWidth-w)/2 + add
		if maxX > mapWidth {
			maxX = mapWidth
		}
	}
	w = maxX - minX
	minHeight := 0
	if crop.PreserveRatio {
		minHeight = int((float64(crop.MinHeight) / float64(crop.MinWidth)) * float64(w))
	}
	if minHeight < crop.MinHeight {
		minHeight = crop.MinHeight
	}
	h := maxY - minY
	if h < minHeight {
		minY -= (minHeight - h) / 2
		add := 0
		if minY < 0 {
			add = -minY
			minY = 0
		}
		maxY += (minHeight-h)/2 + add
		if maxY > mapHeight {
			maxY = mapHeight
		}
	}
	return m.(subImager).SubImage(image.Rect(minX, minY, maxX, maxY))
}

// Pins is like MapPins but uses the embedded world map and pin images.
func Pins(coords []Coord, crop *CropOption) image.Image {
	return MapPins(mapImg, pinParts, coords, crop)
}
