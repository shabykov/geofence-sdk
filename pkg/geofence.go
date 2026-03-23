// Package geofence provides an embeddable, zero-dependency geofencing SDK.
//
// It ships with pre-simplified country and city polygons as generated Go code,
// indexed at init time into an R-tree for microsecond point-in-polygon lookups.
//
// Usage:
//
//	result, err := geofence.Lookup(43.238, 76.945)
//	fmt.Println(result.Country.Name)  // "Kazakhstan"
//	fmt.Println(result.City.Name)     // "Almaty"
//
// To add custom layers:
//
//	geofence.LoadLayer("districts", data, geofence.LayerDistrict)

package pkg

import (
	"fmt"
	"sort"
	"sync"

	"github.com/tidwall/geojson"
	"github.com/tidwall/geojson/geometry"
	"github.com/tidwall/rtree"
)

// ---------- Layer enum ----------

type Layer int

const (
	LayerUnknown  Layer = iota
	LayerCountry        // country boundaries
	LayerRegion         // oblast / state / province
	LayerDistrict       // район / county
	LayerCity           // city boundaries
	LayerCustom         // user-defined
)

var layerNames = map[Layer]string{
	LayerCountry:  "country",
	LayerRegion:   "region",
	LayerDistrict: "district",
	LayerCity:     "city",
	LayerCustom:   "custom",
}

func (l Layer) String() string {
	if s, ok := layerNames[l]; ok {
		return s
	}
	return "unknown"
}

func layerFromString(s string) Layer {
	for l, name := range layerNames {
		if name == s {
			return l
		}
	}
	return LayerUnknown
}

// ---------- Core types ----------

// Zone is a single polygon with metadata, stored in the R-tree.
type Zone struct {
	Object geojson.Object
	Layer  Layer
	Props  Props
}

// Match is a single lookup hit.
type Match struct {
	Name     string `json:"name"`
	Layer    Layer  `json:"layer"`
	ISO      string `json:"iso,omitempty"`
	Currency string `json:"currency,omitempty"`
	Props    Props  `json:"-"`
}

func (m *Match) GetName() string {
	if m == nil {
		return ""
	}
	return m.Name
}

func (m *Match) GetLayer() Layer {
	if m == nil {
		return LayerUnknown
	}
	return Layer(m.Layer)
}

func (m *Match) GetISO() string {
	if m == nil {
		return ""
	}
	return m.ISO
}
func (m *Match) GetCurrency() string {
	if m == nil {
		return ""
	}
	return m.Currency
}

func (m *Match) GetProps() *Props {
	if m == nil {
		return nil
	}
	return &m.Props
}

// Result is the structured output of a Lookup.
type Result struct {
	Country  *Match  `json:"country,omitempty"`
	Region   *Match  `json:"region,omitempty"`
	District *Match  `json:"district,omitempty"`
	City     *Match  `json:"city,omitempty"`
	Custom   []Match `json:"custom,omitempty"`
	All      []Match `json:"all"`
}

// ---------- Engine ----------

// Engine holds the R-tree index and all zones.
type Engine struct {
	mu         sync.RWMutex
	tree       rtree.RTreeG[*Zone]
	zones      []*Zone
	cache      *lruCache
	currencies map[string]string
}

// global singleton, initialized from generated data
var (
	defaultEngine *Engine
	initOnce      sync.Once
	initErr       error
)

// Init explicitly initializes the global engine.
// Called automatically on first Lookup, but you can call it early
// to catch errors at startup.
func Init() error {
	initOnce.Do(func() {
		defaultEngine = &Engine{}
		defaultEngine.cache = newLRUCache(10_000)
		initErr = defaultEngine.loadGenerated()
	})
	return initErr
}

func getEngine() (*Engine, error) {
	if err := Init(); err != nil {
		return nil, err
	}
	return defaultEngine, nil
}

// ---------- Public API (package-level) ----------

// Lookup finds the country, city, and any other zones containing the point.
func Lookup(lat, lng float64) (*Result, error) {
	e, err := getEngine()
	if err != nil {
		return nil, err
	}
	return e.Lookup(lat, lng), nil
}

// Contains returns true if the point is inside any known zone.
func Contains(lat, lng float64) (bool, error) {
	e, err := getEngine()
	if err != nil {
		return false, err
	}
	return e.Contains(lat, lng), nil
}

// Country returns the country name for a point, or "" if not found.
func Country(lat, lng float64) (string, error) {
	r, err := Lookup(lat, lng)
	if err != nil {
		return "", err
	}
	if r.Country != nil {
		return r.Country.Name, nil
	}
	return "", nil
}

// City returns the city name for a point, or "" if not found.
func City(lat, lng float64) (string, error) {
	r, err := Lookup(lat, lng)
	if err != nil {
		return "", err
	}
	if r.City != nil {
		return r.City.Name, nil
	}
	return "", nil
}

// Currency returns the currency code for the country at the given point, or "" if not found.
func Currency(lat, lng float64) (string, error) {
	r, err := Lookup(lat, lng)
	if err != nil {
		return "", err
	}
	if r.Country != nil {
		return r.Country.Currency, nil
	}
	return "", nil
}

// LoadLayer adds a custom GeoJSON layer at runtime.
func LoadLayer(name string, data []byte, layer Layer) error {
	e, err := getEngine()
	if err != nil {
		return err
	}
	return e.LoadLayer(data, layer)
}

// ZoneCount returns total indexed zones.
func ZoneCount() (int, error) {
	e, err := getEngine()
	if err != nil {
		return 0, err
	}
	return e.ZoneCount(), nil
}

// ---------- Engine methods ----------

func (e *Engine) Lookup(lat, lng float64) *Result {
	if e.cache != nil {
		if cached, ok := e.cache.get(lat, lng); ok {
			return cached
		}
	}

	e.mu.RLock()
	matches := e.findAll(lat, lng)
	e.mu.RUnlock()

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Layer > matches[j].Layer
	})

	result := &Result{All: matches}
	for i := range matches {
		m := &matches[i]
		switch m.Layer {
		case LayerCountry:
			if result.Country == nil {
				result.Country = m
			}
		case LayerRegion:
			if result.Region == nil {
				result.Region = m
			}
		case LayerDistrict:
			if result.District == nil {
				result.District = m
			}
		case LayerCity:
			if result.City == nil {
				result.City = m
			}
		case LayerCustom:
			result.Custom = append(result.Custom, *m)
		}
	}

	if e.cache != nil {
		e.cache.put(lat, lng, result)
	}

	return result
}

func (e *Engine) Contains(lat, lng float64) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	point := geometry.Point{X: lng, Y: lat}
	found := false

	e.tree.Search([2]float64{lng, lat}, [2]float64{lng, lat}, func(_, _ [2]float64, zone *Zone) bool {
		if zone.Object.Contains(geojson.NewPoint(point)) {
			found = true
			return false
		}
		return true
	})

	return found
}

func (e *Engine) LoadLayer(data []byte, layer Layer) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.indexGeoJSON(data, layer); err != nil {
		return err
	}
	if e.cache != nil {
		e.cache.clear()
	}
	return nil
}

func (e *Engine) ZoneCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.zones)
}

// ---------- Internal ----------

func (e *Engine) loadGenerated() error {
	// --- Countries ---
	for i := range countries {
		c := &countries[i]

		polys := make([]*geometry.Poly, 0, len(c.Polys))
		for _, polyCoords := range c.Polys {
			if len(polyCoords) == 0 {
				continue
			}
			exterior := make([]geometry.Point, len(polyCoords[0]))
			for j, coord := range polyCoords[0] {
				exterior[j] = geometry.Point{X: coord[0], Y: coord[1]}
			}
			var holes [][]geometry.Point
			for hi := 1; hi < len(polyCoords); hi++ {
				hole := make([]geometry.Point, len(polyCoords[hi]))
				for j, coord := range polyCoords[hi] {
					hole[j] = geometry.Point{X: coord[0], Y: coord[1]}
				}
				holes = append(holes, hole)
			}
			polys = append(polys, geometry.NewPoly(exterior, holes, nil))
		}

		var geom geojson.Object
		if len(polys) == 1 {
			geom = geojson.NewPolygon(polys[0])
		} else {
			geom = geojson.NewMultiPolygon(polys)
		}
		feat := geojson.NewFeature(geom, "")

		zone := &Zone{
			Object: feat,
			Layer:  LayerCountry,
			Props: Props{
				"name":      c.Name,
				"name_long": c.NameLong,
				"iso_a3":    c.ISOA3,
				"iso_a2":    c.ISOA2,
				"layer":     "country",
			},
		}
		rect := feat.Rect()
		e.tree.Insert([2]float64{rect.Min.X, rect.Min.Y}, [2]float64{rect.Max.X, rect.Max.Y}, zone)
		e.zones = append(e.zones, zone)
	}

	// --- Cities ---
	for i := range cities {
		c := &cities[i]

		minLng := c.Lng - c.Half
		minLat := c.Lat - c.Half
		maxLng := c.Lng + c.Half
		maxLat := c.Lat + c.Half

		exterior := []geometry.Point{
			{X: minLng, Y: minLat},
			{X: maxLng, Y: minLat},
			{X: maxLng, Y: maxLat},
			{X: minLng, Y: maxLat},
			{X: minLng, Y: minLat},
		}
		poly := geometry.NewPoly(exterior, nil, nil)
		geom := geojson.NewPolygon(poly)
		feat := geojson.NewFeature(geom, "")

		zone := &Zone{
			Object: feat,
			Layer:  LayerCity,
			Props: Props{
				"name":   c.Name,
				"iso_a3": c.ISOA3,
				"layer":  "city",
			},
		}
		rect := feat.Rect()
		e.tree.Insert([2]float64{rect.Min.X, rect.Min.Y}, [2]float64{rect.Max.X, rect.Max.Y}, zone)
		e.zones = append(e.zones, zone)
	}

	if len(e.zones) == 0 {
		return fmt.Errorf("no zones loaded from generated data")
	}

	e.currencies = defaultCurrencies
	return nil
}

func (e *Engine) indexGeoJSON(data []byte, defaultLayer Layer) error {
	obj, err := geojson.Parse(string(data), nil)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	count := 0
	obj.ForEach(func(feature geojson.Object) bool {
		feat, ok := feature.(*geojson.Feature)
		if !ok {
			return true
		}

		props := parseProps(feat)

		layer := defaultLayer
		if s := props.Str("layer"); s != "" {
			if l := layerFromString(s); l != LayerUnknown {
				layer = l
			}
		}

		zone := &Zone{
			Object: feat,
			Layer:  layer,
			Props:  props,
		}

		rect := feat.Rect()
		e.tree.Insert([2]float64{rect.Min.X, rect.Min.Y}, [2]float64{rect.Max.X, rect.Max.Y}, zone)
		e.zones = append(e.zones, zone)
		count++

		return true
	})

	return nil
}

func (e *Engine) findAll(lat, lng float64) []Match {
	point := geometry.Point{X: lng, Y: lat}
	var matches []Match

	e.tree.Search([2]float64{lng, lat}, [2]float64{lng, lat}, func(_, _ [2]float64, zone *Zone) bool {
		if zone.Object.Contains(geojson.NewPoint(point)) {
			iso := zone.Props.Str("iso_a3")
			matches = append(matches, Match{
				Name:     zone.Props.Str("name"),
				Layer:    zone.Layer,
				ISO:      iso,
				Currency: e.currencies[iso],
				Props:    zone.Props,
			})
		}
		return true
	})

	return matches
}
