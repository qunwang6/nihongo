package helpers

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/gojp/kana"
	elastigo "github.com/mattbaird/elastigo/lib"
	"github.com/revel/revel"
)

func initElasticConnection() elastigo.Conn {
	c := elastigo.NewConn()
	elasticURL, _ := revel.Config.String("elastic.url")
	c.Domain = elasticURL

	elasticPort, found := revel.Config.String("elastic.port")
	if found {
		c.Port = string(elasticPort)
	}
	return *c
}

func Search(query string) (hits [][]byte) {
	c := initElasticConnection()

	query = strings.Replace(query, "\"", "\\\"", -1)

	isLatin := kana.IsLatin(query)
	isKana := kana.IsKana(query)

	// convert to hiragana and katakana
	romaji := kana.KanaToRomaji(query)

	// handle different types of input differently:
	matches := []string{}
	if isKana {
		// add boost for exact-matching kana
		matches = append(matches, fmt.Sprintf(`
		{"match" :
			{
				"furigana" : {
					"query" : "%s",
					"type" : "phrase",
					"boost": 5.0
				}
			}
		}`, query))

		// also look for romaji version in case
		matches = append(matches, fmt.Sprintf(`
		{"match" :
			{
				"romaji" : {
					"query" : "%s",
					"type" : "phrase",
					"boost": 2.0
				}
			}
		}`, romaji))
	}
	if !isLatin {
		matches = append(matches, fmt.Sprintf(`
		{"match" :
			{
				"japanese" : {
					"query" : "%s",
					"type" : "phrase",
					"boost": 10.0
				}
			}
		}`, query))
	} else {
		// add romaji search term
		matches = append(matches, fmt.Sprintf(`
		{"match" :
			{
				"romaji" : {
					"query" : "%s",
					"type" : "phrase",
					"boost": 3.0
				}
			}
		}`, query))

		// add english search term
		matches = append(matches, fmt.Sprintf(`
		{"match" :
			{
				"english" : {
					"query" : "%s",
					"type" : "phrase",
					"boost": 5.0
				}
			}
		}`, query))
	}

	searchJson := fmt.Sprintf(`
		{"query":
			{"bool":
				{
				"should":
					[` + strings.Join(matches, ",") + `],
				"minimum_should_match" : 0,
				"boost": 2.0
				}
			}
		}`)

	out, err := c.Search("edict", "entry", nil, searchJson)
	if err != nil {
		log.Println(err)
	}

	for _, hit := range out.Hits.Hits {
		var h interface{}
		h, err = json.Marshal(&hit.Source)
		if err != nil {
			log.Println(err)
		}

		hits = append(hits, h.([]byte))
	}
	return hits
}

// FuzzySearch returns words similar to the search terms
// provided, and not just exact matches.
func FuzzySearch(query string) (hits [][]byte) {
	c := initElasticConnection()

	searchJson := fmt.Sprintf(`
		{"query":
			{"fuzzy_like_this":
				{
				"fields" : ["romaji", "english"],
				"like_text" : "%s",
				"max_query_terms" : 12
				}
			}
		}`, query)

	out, err := c.Search("edict", "entry", nil, searchJson)
	if err != nil {
		log.Println(err)
	}

	for _, hit := range out.Hits.Hits {
		var h interface{}
		h, err = json.Marshal(&hit.Source)
		if err != nil {
			log.Println(err)
		}

		hits = append(hits, h.([]byte))
	}
	return hits
}
