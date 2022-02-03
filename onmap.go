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
	merkatorImg image.Image
	pin         image.Image
	pinShadow   image.Image
)

// DefaultPinParts are default pin images.
var DefaultPinParts []image.Image

func init() {
	pin = decodeImage(pinData)
	pinShadow = decodeImage(pinShadowData)
	DefaultPinParts = []image.Image{pinShadow, pin}
	merkatorImg = decodeImage(mapData)
	StandardCrop = &CropOption{
		Bound:         100,
		MinWidth:      merkatorImg.Bounds().Max.X / 3,
		MinHeight:     merkatorImg.Bounds().Max.Y / 3,
		PreserveRatio: true,
	}
}

func decodeImage(data []byte) image.Image {
	m, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err.Error())
	}
	return m
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

// Projection is an interface for converting coordinates.
type Projection interface {
	// Convert converts coordinates into a point on a map.
	Convert(coord Coord, mapWidth, mapHeight int) image.Point
}

var Merkator = merkatorProjection(0)

// Merkator implements Projection interface for Merkator projection.
type merkatorProjection int

func (p merkatorProjection) latRad(lat float64) float64 {
	return lat * math.Pi / 180
}

func (p merkatorProjection) n(lat float64) float64 {
	return math.Log(math.Tan((math.Pi / 4) + (p.latRad(lat) / 2)))
}

func (p merkatorProjection) Convert(c Coord, mapWidth, mapHeight int) image.Point {
	mw := float64(mapWidth)
	mh := float64(mapHeight)
	fx := (c.Long + 180) * (mw / 360)
	fy := (mh / 2) - (mw * p.n(c.Lat) / (2 * math.Pi))
	return image.Point{int(math.Round(fx)), int(math.Round(fy))}
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
// World map must be in the given projection.
//
// Pin parts are arbitrary pin images, usually  a shadow of the pin and the pin itself.
// Pin parts are drawn on top of each other from the bottom of the map to the top
// by first drawing pinParts[n], then pinParts[n+1], etc.
// The coordinate point is at the bottom center of each pin part image.
//
func MapPinsProjection(proj Projection, worldMap image.Image, pinParts []image.Image, coords []Coord, crop *CropOption) image.Image {
	mapWidth := worldMap.Bounds().Max.X
	mapHeight := worldMap.Bounds().Max.Y

	cs := make([]image.Point, len(coords))

	// Convert coordinates to x, y.
	for i, c := range coords {
		cs[i] = proj.Convert(c, mapWidth, mapHeight)
	}

	// Sort coordinates by longitude so that
	// lower pins are drawn on top of upper pins.
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Y < cs[j].Y {
			return true
		}
		if cs[i].X < cs[j].X {
			return true
		}
		return false
	})

	// Draw map.
	dc := gg.NewContext(merkatorImg.Bounds().Max.X, merkatorImg.Bounds().Max.Y)
	dc.DrawImage(worldMap, 0, 0)

	// Draw pin parts.
	// Looping over pinParts first to better arrange shadows.
	for _, pin := range pinParts {
		for _, c := range cs {
			dc.DrawImageAnchored(pin, c.X, c.Y, 0.5, 1)
		}
	}

	// Calculate min&max values.
	maxX := 0
	maxY := 0
	minX := mapWidth
	minY := mapHeight
	for _, c := range cs {
		if c.X < minX {
			minX = c.X
		}
		if c.X > maxX {
			maxX = c.X
		}
		if c.Y < minY {
			minY = c.Y
		}
		if c.Y > maxY {
			maxY = c.Y
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

// MapPins is like MapPinsProjection with Merkator projection.
// The world map must be in the same projection.
func MapPins(worldMap image.Image, pinParts []image.Image, coords []Coord, crop *CropOption) image.Image {
	return MapPinsProjection(Merkator, worldMap, pinParts, coords, crop)
}

// Pins is like MapPins but uses the embedded world map and pin images.
func Pins(coords []Coord, crop *CropOption) image.Image {
	return MapPins(merkatorImg, DefaultPinParts, coords, crop)
}
