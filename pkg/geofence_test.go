package pkg

import (
	"sync"
	"testing"
)

func TestInit(t *testing.T) {
	// Reset singleton for test isolation
	initOnce = sync.Once{}
	defaultEngine = nil
	initErr = nil

	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	count, _ := ZoneCount()
	if count == 0 {
		t.Fatal("expected zones > 0 after Init")
	}
	t.Logf("loaded %d zones", count)
}

func TestLookupAlmaty(t *testing.T) {
	resetEngine(t)

	result, err := Lookup(43.25, 76.94)
	if err != nil {
		t.Fatal(err)
	}

	if result.Country == nil {
		t.Fatal("expected country match for Almaty")
	}
	if result.Country.Name != "Kazakhstan" {
		t.Errorf("expected Kazakhstan, got %q", result.Country.Name)
	}
	if result.Country.ISO != "KAZ" {
		t.Errorf("expected KAZ, got %q", result.Country.ISO)
	}

	if result.City == nil {
		t.Fatal("expected city match for Almaty")
	}
	if result.City.Name != "Almaty" {
		t.Errorf("expected Almaty, got %q", result.City.Name)
	}
}

func TestLookupAstana(t *testing.T) {
	resetEngine(t)

	result, err := Lookup(51.13, 71.43)
	if err != nil {
		t.Fatal(err)
	}

	if result.Country == nil || result.Country.Name != "Kazakhstan" {
		t.Errorf("expected Kazakhstan for Astana coords")
	}
	if result.City == nil || result.City.Name != "Astana" {
		t.Errorf("expected Astana city")
	}
}

func TestLookupBerlin(t *testing.T) {
	resetEngine(t)

	result, err := Lookup(52.52, 13.40)
	if err != nil {
		t.Fatal(err)
	}

	if result.Country == nil || result.Country.Name != "Germany" {
		t.Errorf("expected Germany, got %v", result.Country)
	}
	if result.City == nil || result.City.Name != "Berlin" {
		t.Errorf("expected Berlin city, got %v", result.City)
	}
}

func TestLookupTokyo(t *testing.T) {
	resetEngine(t)

	result, err := Lookup(35.6762, 139.6503)
	if err != nil {
		t.Fatal(err)
	}

	if result.Country == nil || result.Country.Name != "Japan" {
		t.Errorf("expected Japan, got %v", result.Country)
	}
	if result.City == nil || result.City.Name != "Tokyo" {
		t.Errorf("expected Tokyo city, got %v", result.City)
	}
}

func TestLookupTurkeyAnkara(t *testing.T) {
	resetEngine(t)

	result, err := Lookup(39.9334, 32.8597)
	if err != nil {
		t.Fatal(err)
	}

	if result.Country == nil || result.Country.Name != "Turkey" {
		t.Errorf("expected Turkey, got %v", result.Country)
	}
	if result.City == nil || result.City.Name != "Ankara" {
		t.Errorf("expected Ankara city, got %v", result.City)
	}
}

func TestLookupOcean(t *testing.T) {
	resetEngine(t)

	result, err := Lookup(0, 0) // Gulf of Guinea
	if err != nil {
		t.Fatal(err)
	}

	if result.Country != nil {
		t.Errorf("expected no country for ocean point, got %q", result.Country.Name)
	}
	if result.City != nil {
		t.Errorf("expected no city for ocean point, got %q", result.City.Name)
	}
}

func TestCountryShortcut(t *testing.T) {
	resetEngine(t)

	name, err := Country(43.25, 76.94)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Kazakhstan" {
		t.Errorf("expected Kazakhstan, got %q", name)
	}
}

func TestCityShortcut(t *testing.T) {
	resetEngine(t)

	name, err := City(43.25, 76.94)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Almaty" {
		t.Errorf("expected Almaty, got %q", name)
	}
}

func TestCurrencyLookup(t *testing.T) {
	resetEngine(t)

	// Almaty → KZT
	r1, err := Lookup(43.25, 76.94)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Country == nil || r1.Country.Currency != "KZT" {
		t.Errorf("expected KZT for Almaty, got %q", r1.Country.Currency)
	}

	// Berlin → EUR
	r2, err := Lookup(52.52, 13.40)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Country == nil || r2.Country.Currency != "EUR" {
		t.Errorf("expected EUR for Berlin, got %q", r2.Country.Currency)
	}
}

func TestCurrencyShortcut(t *testing.T) {
	resetEngine(t)

	cur, err := Currency(43.25, 76.94)
	if err != nil {
		t.Fatal(err)
	}
	if cur != "KZT" {
		t.Errorf("expected KZT, got %q", cur)
	}
}

func TestCurrencyOcean(t *testing.T) {
	resetEngine(t)

	cur, err := Currency(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if cur != "" {
		t.Errorf("expected empty currency for ocean, got %q", cur)
	}
}

func TestContains(t *testing.T) {
	resetEngine(t)

	ok, err := Contains(43.25, 76.94)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected Almaty point to be contained")
	}

	ok, err = Contains(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected ocean point to not be contained")
	}
}

func TestLoadCustomLayer(t *testing.T) {
	resetEngine(t)

	// add a custom district layer
	districtJSON := `{
		"type": "FeatureCollection",
		"features": [{
			"type": "Feature",
			"properties": {"name": "Medeu", "layer": "district"},
			"geometry": {
				"type": "Polygon",
				"coordinates": [[
					[76.90, 43.22], [76.98, 43.22], [76.98, 43.28],
					[76.90, 43.28], [76.90, 43.22]
				]]
			}
		}]
	}`

	err := LoadLayer([]byte(districtJSON), LayerDistrict)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Lookup(43.25, 76.94)
	if err != nil {
		t.Fatal(err)
	}

	if result.District == nil {
		t.Fatal("expected district match after LoadLayer")
	}
	if result.District.Name != "Medeu" {
		t.Errorf("expected Medeu, got %q", result.District.Name)
	}
	// should still have country + city
	if result.Country.Name != "Kazakhstan" {
		t.Error("lost country after LoadLayer")
	}
	if result.City.Name != "Almaty" {
		t.Error("lost city after LoadLayer")
	}
}

func TestCacheBehavior(t *testing.T) {
	resetEngine(t)

	// first call
	r1, _ := Lookup(43.25, 76.94)

	// second call — cache hit
	r2, _ := Lookup(43.25, 76.94)

	if r1.Country.Name != r2.Country.Name {
		t.Error("cache returned inconsistent results")
	}
}

func TestConcurrentLookup(t *testing.T) {
	resetEngine(t)

	points := []struct{ lat, lng float64 }{
		{43.25, 76.94}, // Almaty
		{51.13, 71.43}, // Astana
		{52.52, 13.40}, // Berlin
		{48.86, 2.35},  // Paris
		{0, 0},         // Ocean
	}

	var wg sync.WaitGroup
	errs := make(chan string, 200)

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p := points[idx%len(points)]
			result, err := Lookup(p.lat, p.lng)
			if err != nil {
				errs <- err.Error()
				return
			}
			if p.lat == 43.25 && (result.Country == nil || result.Country.Name != "Kazakhstan") {
				errs <- "bad result for Almaty"
			}
			if p.lat == 0 && result.Country != nil {
				errs <- "bad result for ocean"
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		t.Error(e)
	}
}

func BenchmarkLookup(b *testing.B) {
	resetEngineB(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Lookup(43.25, 76.94)
	}
}

func BenchmarkLookupMiss(b *testing.B) {
	resetEngineB(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Lookup(0, 0)
	}
}

func BenchmarkConcurrent(b *testing.B) {
	resetEngineB(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Lookup(43.25, 76.94)
		}
	})
}

// --- helpers ---

func resetEngine(t *testing.T) {
	t.Helper()
	initOnce = sync.Once{}
	defaultEngine = nil
	initErr = nil
	if err := Init(); err != nil {
		t.Fatal(err)
	}
}

func resetEngineB(b *testing.B) {
	b.Helper()
	initOnce = sync.Once{}
	defaultEngine = nil
	initErr = nil
	if err := Init(); err != nil {
		b.Fatal(err)
	}
}
