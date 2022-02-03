// Package onmap puts pins into a world map image.
package onmap

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sort"
	"sync"
)

//go:embed pin.png
var pinData []byte

//go:embed pin-shadow.png
var pinShadowData []byte

//go:embed mercator.jpg
var mercatorData []byte

var (
	mercatorImg     image.Image
	defaultPinParts []image.Image

	mercatorOnce sync.Once
	pinOnce      sync.Once
)

// DefaultMap returns the default map (Mercator projection).
func DefaultMap() image.Image {
	mercatorOnce.Do(func() {
		mercatorImg = decodeImage(mercatorData)
	})
	return mercatorImg
}

// DefaultPin returns default pin images.
func DefaultPin() []image.Image {
	pinOnce.Do(func() {
		defaultPinParts = []image.Image{decodeImage(pinShadowData), decodeImage(pinData)}
	})
	return defaultPinParts
}

func decodeImage(data []byte) image.Image {
	m, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err.Error())
	}
	return m
}

// StandardCrop is the standard crop.
var StandardCrop *CropOption = &CropOption{
	Bound:         100,
	MinWidth:      640,
	MinHeight:     543,
	PreserveRatio: true,
}

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

// Mercator provides the Mercator projection.
var Mercator = mercatorProjection(0)

type mercatorProjection int

func (p mercatorProjection) latRad(lat float64) float64 {
	return lat * math.Pi / 180
}

func (p mercatorProjection) n(lat float64) float64 {
	return math.Log(math.Tan((math.Pi / 4) + (p.latRad(lat) / 2)))
}

func (p mercatorProjection) Convert(c Coord, mapWidth, mapHeight int) image.Point {
	mw := float64(mapWidth)
	mh := float64(mapHeight)
	fx := (c.Long + 180) * (mw / 360)
	fy := (mh / 2) - (mw * p.n(c.Lat) / (2 * math.Pi))
	return image.Point{int(math.Round(fx)), int(math.Round(fy))}
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

	// Sort coordinates by latitude so that
	// lower pins are drawn on top of upper pins.
	sort.Slice(cs, func(i, j int) bool {
		return cs[i].Y < cs[j].Y
	})

	// Draw map.
	m := image.NewRGBA(image.Rect(0, 0, worldMap.Bounds().Dx(), worldMap.Bounds().Dy()))
	draw.Draw(m, m.Bounds(), worldMap, worldMap.Bounds().Min, draw.Over)

	// Draw pin parts.
	// Looping over pinParts first to better arrange shadows.
	for _, pin := range pinParts {
		halfw := pin.Bounds().Dx() / 2
		h := pin.Bounds().Dy()
		min := pin.Bounds().Min
		for _, c := range cs {
			r := image.Rect(c.X-halfw, c.Y-h, c.X+halfw, c.Y)
			draw.Draw(m, r, pin, min, draw.Over)
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
	return m.SubImage(image.Rect(minX, minY, maxX, maxY))
}

// MapPins is like MapPinsProjection with Mercator projection.
// The world map must be in the same projection.
func MapPins(worldMap image.Image, pinParts []image.Image, coords []Coord, crop *CropOption) image.Image {
	return MapPinsProjection(Mercator, worldMap, pinParts, coords, crop)
}

// Pins is like MapPins but uses the embedded world map and pin images.
func Pins(coords []Coord, crop *CropOption) image.Image {
	return MapPins(DefaultMap(), DefaultPin(), coords, crop)
}
