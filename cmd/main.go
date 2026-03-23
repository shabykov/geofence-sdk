// Command codegen downloads Natural Earth data, filters and simplifies it,
// and generates Go source files with embedded geodata.
//
// Usage:
//
//	go run ./cmd/main.go
//
// It produces:
//   - pkg/gen_countries.go  (country polygon data as Go literals)
//   - pkg/gen_cities.go     (city bounding rectangles as Go literals)
//   - pkg/gen_currencies.go (ISO→currency map as Go literal)
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ---------- configuration ----------

// Countries to include (ISO_A3 codes).
var wantedCountries = map[string]bool{
	// Central Asia
	"KAZ": true, "UZB": true, "KGZ": true, "TJK": true, "TKM": true,
	// East Asia
	"CHN": true, "JPN": true, "KOR": true, "PRK": true, "MNG": true, "TWN": true,
	// Southeast Asia
	"VNM": true, "THA": true, "MMR": true, "KHM": true, "LAO": true,
	"MYS": true, "IDN": true, "PHL": true, "SGP": true, "BRN": true, "TLS": true,
	// South Asia
	"IND": true, "PAK": true, "BGD": true, "LKA": true, "NPL": true,
	"BTN": true, "MDV": true, "AFG": true,
	// Middle East
	"IRN": true, "IRQ": true, "SYR": true, "JOR": true, "LBN": true,
	"ISR": true, "PSE": true, "SAU": true, "YEM": true, "OMN": true,
	"ARE": true, "QAT": true, "BHR": true, "KWT": true,
	// Caucasus
	"GEO": true, "ARM": true, "AZE": true,
	// Turkey
	"TUR": true,
	// Russia
	"RUS": true,
	// Europe — Western
	"GBR": true, "IRL": true, "FRA": true, "NLD": true, "BEL": true, "LUX": true,
	"DEU": true, "AUT": true, "CHE": true, "LIE": true, "MCO": true,
	// Europe — Southern
	"ESP": true, "PRT": true, "ITA": true, "GRC": true, "CYP": true,
	"MLT": true, "AND": true, "SMR": true,
	// Europe — Northern
	"SWE": true, "NOR": true, "FIN": true, "DNK": true, "ISL": true,
	// Europe — Eastern
	"POL": true, "CZE": true, "SVK": true, "HUN": true, "ROU": true,
	"BGR": true, "UKR": true, "BLR": true, "MDA": true,
	"EST": true, "LVA": true, "LTU": true,
	// Europe — Balkans
	"HRV": true, "SRB": true, "SVN": true, "BIH": true,
	"MNE": true, "ALB": true, "MKD": true, "XKX": true,
	// North Africa
	"MAR": true, "DZA": true, "TUN": true, "LBY": true, "EGY": true,
	// West Africa
	"NGA": true, "GHA": true, "CIV": true, "SEN": true, "MLI": true,
	"BFA": true, "NER": true, "GIN": true, "SLE": true, "LBR": true,
	"TGO": true, "BEN": true, "GMB": true, "GNB": true, "CPV": true, "MRT": true,
	// Central Africa
	"CMR": true, "CAF": true, "TCD": true, "COG": true, "COD": true,
	"GAB": true, "GNQ": true, "STP": true,
	// East Africa
	"ETH": true, "KEN": true, "TZA": true, "UGA": true, "RWA": true,
	"BDI": true, "SOM": true, "DJI": true, "ERI": true, "SSD": true, "SDN": true,
	// Southern Africa
	"ZAF": true, "NAM": true, "BWA": true, "ZMB": true, "ZWE": true,
	"MOZ": true, "MWI": true, "MDG": true, "SWZ": true, "LSO": true,
	"AGO": true, "MUS": true, "COM": true, "SYC": true,
	// North America
	"USA": true, "CAN": true, "MEX": true,
	// Central America
	"GTM": true, "BLZ": true, "HND": true, "SLV": true, "NIC": true,
	"CRI": true, "PAN": true,
	// Caribbean
	"CUB": true, "HTI": true, "DOM": true, "JAM": true, "TTO": true,
	"BHS": true, "BRB": true, "GRD": true, "DMA": true,
	"LCA": true, "VCT": true, "KNA": true, "ATG": true,
	// South America
	"BRA": true, "ARG": true, "CHL": true, "COL": true, "PER": true,
	"VEN": true, "ECU": true, "BOL": true, "PRY": true, "URY": true,
	"GUY": true, "SUR": true,
	// Oceania
	"AUS": true, "NZL": true, "PNG": true, "FJI": true,
	"SLB": true, "VUT": true, "WSM": true, "TON": true,
	"KIR": true, "FSM": true, "MHL": true, "PLW": true,
	"NRU": true, "TUV": true,
	// Territories
	"GRL": true, "NCL": true, "PYF": true, "FLK": true,
	"ESH": true, "PRI": true, "HKG": true, "MAC": true,
}

const naturalEarthURL = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_110m_admin_0_countries.geojson"
const naturalEarthCitiesURL = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_10m_populated_places_simple.geojson"
const minCityPopulation = 5_000
const simplifyTolerance = 0.01

// currencies maps ISO_A3 country codes to ISO currency codes.
var currencies = map[string]string{
	"AFG": "AFN", "ALB": "ALL", "DZA": "DZD", "AND": "EUR", "AGO": "AOA",
	"ATG": "XCD", "ARG": "ARS", "ARM": "AMD", "AUS": "AUD", "AUT": "EUR",
	"AZE": "AZN", "BHS": "BSD", "BHR": "BHD", "BGD": "BDT", "BRB": "BBD",
	"BLR": "BYN", "BEL": "EUR", "BLZ": "BZD", "BEN": "XOF", "BTN": "BTN",
	"BOL": "BOB", "BIH": "BAM", "BWA": "BWP", "BRA": "BRL", "BRN": "BND",
	"BGR": "BGN", "BFA": "XOF", "BDI": "BIF", "CPV": "CVE", "KHM": "KHR",
	"CMR": "XAF", "CAN": "CAD", "CAF": "XAF", "TCD": "XAF", "CHL": "CLP",
	"CHN": "CNY", "COL": "COP", "COM": "KMF", "COG": "XAF", "COD": "CDF",
	"CRI": "CRC", "CIV": "XOF", "HRV": "EUR", "CUB": "CUP", "CYP": "EUR",
	"CZE": "CZK", "DNK": "DKK", "DJI": "DJF", "DMA": "XCD", "DOM": "DOP",
	"ECU": "USD", "EGY": "EGP", "SLV": "USD", "GNQ": "XAF", "ERI": "ERN",
	"EST": "EUR", "SWZ": "SZL", "ETH": "ETB", "FJI": "FJD", "FIN": "EUR",
	"FRA": "EUR", "GAB": "XAF", "GMB": "GMD", "GEO": "GEL", "DEU": "EUR",
	"GHA": "GHS", "GRC": "EUR", "GRD": "XCD", "GTM": "GTQ", "GIN": "GNF",
	"GNB": "XOF", "GUY": "GYD", "HTI": "HTG", "HND": "HNL", "HUN": "HUF",
	"ISL": "ISK", "IND": "INR", "IDN": "IDR", "IRN": "IRR", "IRQ": "IQD",
	"IRL": "EUR", "ISR": "ILS", "ITA": "EUR", "JAM": "JMD", "JPN": "JPY",
	"JOR": "JOD", "KAZ": "KZT", "KEN": "KES", "KIR": "AUD", "PRK": "KPW",
	"KOR": "KRW", "KWT": "KWD", "KGZ": "KGS", "LAO": "LAK", "LVA": "EUR",
	"LBN": "LBP", "LSO": "LSL", "LBR": "LRD", "LBY": "LYD", "LIE": "CHF",
	"LTU": "EUR", "LUX": "EUR", "MDG": "MGA", "MWI": "MWK", "MYS": "MYR",
	"MDV": "MVR", "MLI": "XOF", "MLT": "EUR", "MHL": "USD", "MRT": "MRU",
	"MUS": "MUR", "MEX": "MXN", "FSM": "USD", "MDA": "MDL", "MCO": "EUR",
	"MNG": "MNT", "MNE": "EUR", "MAR": "MAD", "MOZ": "MZN", "MMR": "MMK",
	"NAM": "NAD", "NRU": "AUD", "NPL": "NPR", "NLD": "EUR", "NZL": "NZD",
	"NIC": "NIO", "NER": "XOF", "NGA": "NGN", "MKD": "MKD", "NOR": "NOK",
	"OMN": "OMR", "PAK": "PKR", "PLW": "USD", "PAN": "PAB", "PNG": "PGK",
	"PRY": "PYG", "PER": "PEN", "PHL": "PHP", "POL": "PLN", "PRT": "EUR",
	"QAT": "QAR", "ROU": "RON", "RUS": "RUB", "RWA": "RWF", "KNA": "XCD",
	"LCA": "XCD", "VCT": "XCD", "WSM": "WST", "SMR": "EUR", "STP": "STN",
	"SAU": "SAR", "SEN": "XOF", "SRB": "RSD", "SYC": "SCR", "SLE": "SLE",
	"SGP": "SGD", "SVK": "EUR", "SVN": "EUR", "SLB": "SBD", "SOM": "SOS",
	"ZAF": "ZAR", "SSD": "SSP", "ESP": "EUR", "LKA": "LKR", "SDN": "SDG",
	"SUR": "SRD", "SWE": "SEK", "CHE": "CHF", "SYR": "SYP", "TWN": "TWD",
	"TJK": "TJS", "TZA": "TZS", "THA": "THB", "TLS": "USD", "TGO": "XOF",
	"TON": "TOP", "TTO": "TTD", "TUN": "TND", "TUR": "TRY", "TKM": "TMT",
	"TUV": "AUD", "UGA": "UGX", "UKR": "UAH", "ARE": "AED", "GBR": "GBP",
	"USA": "USD", "URY": "UYU", "UZB": "UZS", "VUT": "VUV", "VEN": "VES",
	"VNM": "VND", "YEM": "YER", "ZMB": "ZMW", "ZWE": "ZWL", "PSE": "ILS",
	"XKX": "EUR", "ESH": "MAD", "SOL": "SBD", "CUW": "ANG", "SXM": "ANG",
	"NCL": "XPF", "PYF": "XPF", "GUF": "EUR", "GLP": "EUR", "MTQ": "EUR",
	"REU": "EUR", "MYT": "EUR", "FLK": "FKP", "GRL": "DKK", "FRO": "DKK",
	"PRI": "USD", "HKG": "HKD", "MAC": "MOP",
}

func main() {
	outDir := "../pkg"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	// --- Step 1: Download and process countries ---
	log.Println("downloading Natural Earth 110m countries...")
	raw, err := download(naturalEarthURL)
	if err != nil {
		log.Fatalf("download: %v", err)
	}
	log.Printf("downloaded %d bytes", len(raw))

	var fc FeatureCollection
	if err := json.Unmarshal(raw, &fc); err != nil {
		log.Fatalf("parse: %v", err)
	}

	var filtered []Feature
	for _, f := range fc.Features {
		iso := f.propStr("ISO_A3")
		if iso == "" {
			iso = f.propStr("ISO_A3_EH")
		}
		if !wantedCountries[iso] {
			continue
		}
		f.Geometry = simplifyGeometry(f.Geometry, simplifyTolerance)
		f.Properties = map[string]any{
			"iso_a3":    iso,
			"iso_a2":    firstNonEmpty(f.propStr("ISO_A2"), f.propStr("ISO_A2_EH")),
			"name":      f.propStr("NAME"),
			"name_long": f.propStr("NAME_LONG"),
			"layer":     "country",
		}
		filtered = append(filtered, f)
	}
	log.Printf("filtered %d -> %d countries", len(fc.Features), len(filtered))

	if err := generateCountriesGo(outDir, filtered); err != nil {
		log.Fatalf("generateCountriesGo: %v", err)
	}

	// --- Step 2: Download and process cities ---
	cities, err := downloadCities()
	if err != nil {
		log.Fatalf("downloadCities: %v", err)
	}
	log.Printf("filtered %d cities (pop >= %d)", len(cities), minCityPopulation)

	if err := generateCitiesGo(outDir, cities); err != nil {
		log.Fatalf("generateCitiesGo: %v", err)
	}

	// --- Step 3: Generate currencies ---
	if err := generateCurrenciesGo(outDir); err != nil {
		log.Fatalf("generateCurrenciesGo: %v", err)
	}

	log.Println("done! Generated Go files in", outDir)
}

// ---------- types ----------

type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Feature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Geometry   Geometry       `json:"geometry"`
}

func (f Feature) propStr(key string) string {
	if v, ok := f.Properties[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (f Feature) propFloat(key string) float64 {
	if v, ok := f.Properties[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return 0
}

type Geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

type Point [2]float64

// cityEntry holds processed city data for code generation.
type cityEntry struct {
	Name  string
	ISOA3 string
	Lng   float64
	Lat   float64
	Half  float64
}

// ---------- Go source generation ----------

func generateCountriesGo(outDir string, features []Feature) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by cmd/main.go; DO NOT EDIT.\n\npackage pkg\n\n")
	buf.WriteString("type countryDef struct {\n")
	buf.WriteString("\tName     string\n")
	buf.WriteString("\tNameLong string\n")
	buf.WriteString("\tISOA3    string\n")
	buf.WriteString("\tISOA2    string\n")
	buf.WriteString("\tPolys    [][][][2]float64\n")
	buf.WriteString("}\n\n")
	buf.WriteString("var countries = []countryDef{\n")

	for _, f := range features {
		name := f.propStr("name")
		nameLong := f.propStr("name_long")
		isoA3 := f.propStr("iso_a3")
		isoA2 := f.propStr("iso_a2")

		polys := normalizeToMultiPolygon(f.Geometry)

		fmt.Fprintf(&buf, "\t{Name: %q, NameLong: %q, ISOA3: %q, ISOA2: %q,\n",
			name, nameLong, isoA3, isoA2)
		buf.WriteString("\t\tPolys: [][][][2]float64{\n")

		for _, poly := range polys {
			buf.WriteString("\t\t\t{\n")
			for _, ring := range poly {
				buf.WriteString("\t\t\t\t{")
				for j, pt := range ring {
					if j > 0 {
						buf.WriteString(", ")
					}
					buf.WriteByte('{')
					buf.WriteString(fmtFloat(pt[0]))
					buf.WriteString(", ")
					buf.WriteString(fmtFloat(pt[1]))
					buf.WriteByte('}')
				}
				buf.WriteString("},\n")
			}
			buf.WriteString("\t\t\t},\n")
		}
		buf.WriteString("\t\t}},\n")
	}

	buf.WriteString("}\n")
	return writeGoFile(filepath.Join(outDir, "gen_countries.go"), buf.Bytes())
}

func generateCitiesGo(outDir string, cities []cityEntry) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by cmd/main.go; DO NOT EDIT.\n\npackage pkg\n\n")
	buf.WriteString("type cityDef struct {\n")
	buf.WriteString("\tName  string\n")
	buf.WriteString("\tISOA3 string\n")
	buf.WriteString("\tLng   float64\n")
	buf.WriteString("\tLat   float64\n")
	buf.WriteString("\tHalf  float64\n")
	buf.WriteString("}\n\n")
	buf.WriteString("var cities = []cityDef{\n")

	for _, c := range cities {
		fmt.Fprintf(&buf, "\t{%q, %q, %s, %s, %s},\n",
			c.Name, c.ISOA3,
			fmtFloat(c.Lng), fmtFloat(c.Lat), fmtFloat(c.Half))
	}

	buf.WriteString("}\n")
	return writeGoFile(filepath.Join(outDir, "gen_cities.go"), buf.Bytes())
}

func generateCurrenciesGo(outDir string) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by cmd/main.go; DO NOT EDIT.\n\npackage pkg\n\n")
	buf.WriteString("var defaultCurrencies = map[string]string{\n")

	// Sort keys for deterministic output
	keys := make([]string, 0, len(currencies))
	for k := range currencies {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(&buf, "\t%q: %q,\n", k, currencies[k])
	}

	buf.WriteString("}\n")
	return writeGoFile(filepath.Join(outDir, "gen_currencies.go"), buf.Bytes())
}

// ---------- city download ----------

func cityBoxSize(pop float64) float64 {
	switch {
	case pop >= 10_000_000:
		return 0.20
	case pop >= 5_000_000:
		return 0.15
	case pop >= 1_000_000:
		return 0.10
	default:
		return 0.07
	}
}

func downloadCities() ([]cityEntry, error) {
	log.Println("downloading Natural Earth populated places...")
	raw, err := download(naturalEarthCitiesURL)
	if err != nil {
		return nil, fmt.Errorf("download cities: %w", err)
	}
	log.Printf("downloaded %d bytes", len(raw))

	var fc FeatureCollection
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("parse cities: %w", err)
	}

	var result []cityEntry
	for _, f := range fc.Features {
		pop := f.propFloat("pop_max")
		if pop < minCityPopulation {
			continue
		}
		iso := f.propStr("adm0_a3")
		if !wantedCountries[iso] {
			continue
		}
		var coords [2]float64
		if err := json.Unmarshal(f.Geometry.Coordinates, &coords); err != nil {
			continue
		}
		name := f.propStr("name")
		if name == "Nur-Sultan" {
			name = "Astana"
		}
		result = append(result, cityEntry{
			Name:  name,
			ISOA3: iso,
			Lng:   coords[0],
			Lat:   coords[1],
			Half:  cityBoxSize(pop),
		})
	}
	return result, nil
}

// ---------- simplification ----------

func simplifyGeometry(g Geometry, tolerance float64) Geometry {
	switch g.Type {
	case "Polygon":
		var rings [][]Point
		json.Unmarshal(g.Coordinates, &rings)
		var valid [][]Point
		for _, ring := range rings {
			s := douglasPeucker(ring, tolerance)
			if len(s) >= 4 {
				valid = append(valid, s)
			}
		}
		if len(valid) > 0 {
			g.Coordinates, _ = json.Marshal(valid)
		}
	case "MultiPolygon":
		var polys [][][]Point
		json.Unmarshal(g.Coordinates, &polys)
		var validPolys [][][]Point
		for _, poly := range polys {
			var valid [][]Point
			for _, ring := range poly {
				s := douglasPeucker(ring, tolerance)
				if len(s) >= 4 {
					valid = append(valid, s)
				}
			}
			if len(valid) > 0 {
				validPolys = append(validPolys, valid)
			}
		}
		if len(validPolys) > 0 {
			g.Coordinates, _ = json.Marshal(validPolys)
		}
	}
	return g
}

func douglasPeucker(points []Point, epsilon float64) []Point {
	if len(points) <= 2 {
		return points
	}
	dmax := 0.0
	idx := 0
	end := len(points) - 1
	for i := 1; i < end; i++ {
		d := perpendicularDist(points[i], points[0], points[end])
		if d > dmax {
			dmax = d
			idx = i
		}
	}
	if dmax > epsilon {
		left := douglasPeucker(points[:idx+1], epsilon)
		right := douglasPeucker(points[idx:], epsilon)
		return append(left[:len(left)-1], right...)
	}
	return []Point{points[0], points[end]}
}

func perpendicularDist(p, a, b Point) float64 {
	dx := b[0] - a[0]
	dy := b[1] - a[1]
	if dx == 0 && dy == 0 {
		return math.Hypot(p[0]-a[0], p[1]-a[1])
	}
	t := ((p[0]-a[0])*dx + (p[1]-a[1])*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	projX := a[0] + t*dx
	projY := a[1] + t*dy
	return math.Hypot(p[0]-projX, p[1]-projY)
}

// ---------- helpers ----------

func normalizeToMultiPolygon(g Geometry) [][][]Point {
	switch g.Type {
	case "Polygon":
		var rings [][]Point
		json.Unmarshal(g.Coordinates, &rings)
		return [][][]Point{rings}
	case "MultiPolygon":
		var polys [][][]Point
		json.Unmarshal(g.Coordinates, &polys)
		return polys
	default:
		return nil
	}
}

func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func writeGoFile(path string, src []byte) error {
	formatted, err := format.Source(src)
	if err != nil {
		log.Printf("warning: gofmt failed for %s: %v (writing unformatted)", path, err)
		formatted = src
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return err
	}
	kb := float64(len(formatted)) / 1024
	log.Printf("  %s: %.1f KB", path, kb)
	return nil
}

func download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
