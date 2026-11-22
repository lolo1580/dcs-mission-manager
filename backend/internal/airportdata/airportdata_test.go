package airportdata

import "testing"

func TestEmbeddedDatasetSearch(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Count() < 40000 {
		t.Fatalf("dataset too small: %d", catalog.Count())
	}
	cases := []struct {
		query string
		want  string
	}{
		{"LFBO", "LFBO"},
		{"Toulouse", "LFBO"},
		{"JFK", "KJFK"},
		{"zurich", "LSZH"},
	}
	for _, tc := range cases {
		results := catalog.Search(tc.query, 5)
		if len(results) == 0 {
			t.Fatalf("%s: no result", tc.query)
		}
		found := false
		for _, result := range results {
			if result.ICAO == tc.want || result.Ident == tc.want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: %q not in %+v", tc.query, tc.want, results)
		}
	}
	if results := catalog.Search("zzzzzz", 5); len(results) != 0 {
		t.Fatalf("unexpected result: %+v", results)
	}
}

func TestScoreQueryRanksExactIdentFirst(t *testing.T) {
	exact := Airport{Ident: "LFBO", ICAO: "LFBO", Name: "Toulouse", Type: "large"}
	contains := Airport{Ident: "XLFBO", ICAO: "", Name: "Elsewhere", Type: "small"}
	if ScoreQuery("LFBO", exact) <= ScoreQuery("LFBO", contains) {
		t.Fatal("exact ident must outrank a prefix match")
	}
	if ScoreQuery("", exact) != 0 {
		t.Fatal("empty query must not match")
	}
}

func TestDedupePrefersDCSAirfield(t *testing.T) {
	base := []Result{{
		Airport: Airport{Ident: "LFBO", ICAO: "LFBO", Name: "Toulouse", Source: "ourairports"},
		Score:   1000,
	}}
	extra := []Result{{
		Airport: Airport{Ident: "LFBO", ICAO: "LFBO", Name: "Toulouse DCS", Source: "dcs", Tower: 118.1},
		Score:   1010,
	}}
	merged := Dedupe(base, extra, 10)
	if len(merged) != 1 {
		t.Fatalf("merged = %+v", merged)
	}
	if merged[0].Source != "dcs" || merged[0].Tower != 118.1 {
		t.Fatalf("DCS entry did not win: %+v", merged[0])
	}

	unique := Result{Airport: Airport{Ident: "BATUMI", Name: "Batumi", Source: "dcs"}, Score: 900}
	merged = Dedupe(base, []Result{unique}, 10)
	if len(merged) != 2 {
		t.Fatalf("unique DCS airfield lost: %+v", merged)
	}
}
