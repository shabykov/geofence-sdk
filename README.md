# geofence — Embeddable Geofencing SDK for Go

Zero-dependency\* client-side geofencing with pre-bundled country and city polygons.
Single `go get`, no external API calls, microsecond lookups.

> \* Only depends on `tidwall/geojson` + `tidwall/rtree` + `hashicorp/golang-lru` for the spatial index.

Originally an internal package, now open-sourced and free to use.

## Quick Start

```go
import geofence "github.com/shabykov/geofence-sdk/pkg"

func main() {
    result, err := geofence.Lookup(43.238, 76.945)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Country.Name)     // "Kazakhstan"
    fmt.Println(result.Country.ISO)      // "KAZ"
    fmt.Println(result.Country.Currency) // "KZT"
    fmt.Println(result.City.Name)        // "Almaty"
}
```

## Public API

### Functions

```go
// Init explicitly initializes the engine.
// Called automatically on first Lookup, but can be called early to catch errors at startup.
func Init() error

// Lookup returns all matching zones (country, city, region, district, custom) for a point.
func Lookup(lat, lng float64) (*Result, error)

// Country returns the country name for a point, or "" if not found.
func Country(lat, lng float64) (string, error)

// City returns the city name for a point, or "" if not found.
func City(lat, lng float64) (string, error)

// Currency returns the ISO currency code for the country at the given point, or "" if not found.
func Currency(lat, lng float64) (string, error)

// Contains returns true if the point is inside any known zone.
func Contains(lat, lng float64) (bool, error)

// LoadLayer adds a custom GeoJSON FeatureCollection at runtime.
// Supports layers: LayerCountry, LayerRegion, LayerDistrict, LayerCity, LayerCustom.
func LoadLayer(data []byte, layer Layer) error

// ZoneCount returns the total number of indexed zones.
func ZoneCount() (int, error)
```

### Types

```go
// Result is the structured output of Lookup.
type Result struct {
    Country  *Match  // best country match or nil
    Region   *Match  // best region match or nil
    District *Match  // best district match or nil
    City     *Match  // best city match or nil
    Custom   []Match // all custom layer matches
    All      []Match // all matches across all layers
}

// Match is a single lookup hit.
type Match struct {
    Name     string // "Kazakhstan", "Almaty", etc.
    Layer    Layer  // LayerCountry, LayerCity, etc.
    ISO      string // ISO 3166-1 alpha-3 ("KAZ")
    Currency string // ISO 4217 currency code ("KZT")
    Props    Props  // all properties as map[string]any
}

// Layer identifies the type of geographic zone.
type Layer int

const (
    LayerUnknown  Layer = iota
    LayerCountry        // country boundaries
    LayerRegion         // oblast / state / province
    LayerDistrict       // district / county
    LayerCity           // city boundaries
    LayerCustom         // user-defined
)

// Props provides typed access to feature properties.
type Props map[string]any

func (p Props) Str(key string) string
func (p Props) Int(key string) int
func (p Props) Float(key string) float64
func (p Props) Has(key string) bool
```

### Usage Examples

```go
// Full lookup
result, _ := geofence.Lookup(52.52, 13.40)
fmt.Println(result.Country.Name) // "Germany"
fmt.Println(result.City.Name)    // "Berlin"
fmt.Println(result.Country.Currency) // "EUR"

// Shortcuts
country, _ := geofence.Country(43.25, 76.94)  // "Kazakhstan"
city, _    := geofence.City(43.25, 76.94)     // "Almaty"
cur, _     := geofence.Currency(43.25, 76.94) // "KZT"
ok, _      := geofence.Contains(43.25, 76.94) // true

// Runtime custom layer
data, _ := os.ReadFile("districts.geojson")
geofence.LoadLayer(data, geofence.LayerDistrict)

result, _ = geofence.Lookup(43.25, 76.94)
fmt.Println(result.District.Name) // "Medeu"

// Access raw properties
fmt.Println(result.Country.Props.Str("iso_a2"))    // "KZ"
fmt.Println(result.Country.Props.Str("name_long")) // "Kazakhstan"
```

## Bundled Data

| Layer      | Source                   | Coverage      | Zones |
|------------|--------------------------|---------------|-------|
| Countries  | Natural Earth 110m       | 170 countries | 170   |
| Cities     | Natural Earth 10m places | pop >= 5K     | ~6000 |
| Currencies | Inline ISO mapping       | 214 codes     | —     |

## Updating Geodata

Geodata is stored as generated Go source files (`pkg/gen_*.go`), not as embedded JSON.
To regenerate after changing configuration:

```bash
make generate   # or: go run ./cmd/main.go
```

This will:
1. Download Natural Earth 110m countries and 10m populated places
2. Filter by `wantedCountries` list and `minCityPopulation` threshold
3. Simplify country polygons (Douglas-Peucker)
4. Generate `pkg/gen_countries.go`, `pkg/gen_cities.go`, `pkg/gen_currencies.go`

### Adding/removing countries

Edit `wantedCountries` in `cmd/main.go` (ISO_A3 codes), then `make generate`.

### Changing city population threshold

Edit `minCityPopulation` in `cmd/main.go`, then `make generate`.

## Architecture

```
Lookup(lat, lng)
    │
    ├─ LRU Cache hit? → return cached Result
    │
    ├─ R-tree bbox filter (O(log n))
    │       │
    │       └─ Point-in-polygon check (candidates only)
    │
    └─ Sort by Layer (City > District > Region > Country)
           │
           └─ Build Result { Country, Region, District, City, Custom }
```

- Geodata compiled into binary as Go literals — zero I/O, no JSON parsing at init
- R-tree from `tidwall/rtree` — one of the fastest spatial indexes in Go
- Zones built at init via `tidwall/geojson` programmatic constructors
- LRU `hashicorp/golang-lru` with ~11m grid quantization (10K entries default) and ttl (1 hour)

## Makefile

```bash
make generate   # download data + generate Go source files
make test       # run tests
make bench      # run benchmarks
```
