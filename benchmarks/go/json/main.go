// Hand-written Go port of benchmarks/json.rb, for the README comparison column.
// encoding/json with typed structs for generate; Float gets Ruby's "1250.0" form via a MarshalJSON.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
)

// rubyFloat marshals like Ruby's JSON: Float#to_s, so 1250.0 keeps its ".0".
type rubyFloat float64

func (f rubyFloat) MarshalJSON() ([]byte, error) {
	b := strconv.AppendFloat(nil, float64(f), 'f', -1, 64)
	for _, c := range b {
		if c == '.' {
			return b, nil
		}
	}
	return append(b, ".0"...), nil
}

type Meta struct {
	N     *int  `json:"n"`
	Depth []any `json:"depth"`
}

type Record struct {
	ID    int       `json:"id"`
	Name  string    `json:"name"`
	Tags  []string  `json:"tags"`
	Price rubyFloat `json:"price"`
	OK    bool      `json:"ok"`
	Meta  Meta      `json:"meta"`
}

func main() {
	records := make([]Record, 5_000)
	for i := range records {
		records[i] = Record{
			ID:    i,
			Name:  "item " + strconv.Itoa(i),
			Tags:  []string{"t" + strconv.Itoa(i%7), "t" + strconv.Itoa(i%11)},
			Price: rubyFloat(float64(i) * 1.25),
			OK:    i%2 == 0,
			Meta:  Meta{Depth: []any{1, []any{2, []any{3}}}},
		}
	}

	total := 0
	for range 10 {
		text, err := json.Marshal(records)
		if err != nil {
			log.Fatal(err)
		}
		var back []any
		if err := json.Unmarshal(text, &back); err != nil {
			log.Fatal(err)
		}
		total += len(back) + len(text)
	}
	fmt.Println(total)

	pretty, err := json.MarshalIndent(records[:2], "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(strings.Count(string(pretty), "\n") + 1)
	var parsed []map[string]any
	if err := json.Unmarshal(pretty, &parsed); err != nil {
		log.Fatal(err)
	}
	tags := parsed[len(parsed)-1]["tags"].([]any)
	parts := make([]string, len(tags))
	for i, t := range tags {
		parts[i] = strconv.Quote(t.(string))
	}
	fmt.Println("[" + strings.Join(parts, ", ") + "]")
}
