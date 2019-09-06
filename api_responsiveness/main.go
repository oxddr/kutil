package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"strconv"
	"time"
)

const (
	epsilon     = 0.00001
	minDuration = 50 * time.Millisecond
)

var (
	threshold = flag.Float64("threshold", 0.1, "Failure threshold.")
	mode      = flag.String("mode", "compare", "Mode")
	baseline  = flag.String("baseline", "", "")
)

type APIResponsiveness struct {
	DataItems []Item `json:"DataItems"`
}

type Item struct {
	Data   Data   `json:"Data"`
	Labels Labels `json:"labels"`
}

type Data struct {
	Perc99 float64 `json:"Perc99"`
}

type Labels struct {
	Resource    string `json:"Resource"`
	Scope       string `json:"Scope"`
	Subresource string `json:"Subresource"`
	Verb        string `json:"Verb"`
	Count       string `json:"Count"`
}

func (l *Labels) asKey() string {
	key := fmt.Sprintf("%s %s/%s", l.Verb, l.Scope, l.Resource)
	if l.Subresource != "" {
		key = fmt.Sprintf("%s/%s", key, l.Subresource)
	}
	return key
}

func (l *Labels) count() int {
	i, err := strconv.Atoi(l.Count)
	if err != nil {
		log.Fatalf("cannot convert count: %s", l.Count)
	}
	return i
}

func (d *APIResponsiveness) asMap() map[string]time.Duration {
	m := make(map[string]time.Duration)
	for _, item := range d.DataItems {
		if item.Labels.Scope != "" && item.Data.Perc99 > epsilon {
			// APIResponsiveness kee
			m[item.Labels.asKey()] = time.Millisecond * time.Duration(item.Data.Perc99)
		}
	}
	return m
}

func parseResults(path string) (*APIResponsiveness, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result APIResponsiveness
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func compareResults(base, result *APIResponsiveness) {
	b := base.asMap()
	r := result.asMap()

	good := 0
	bad := 0

	for k, v := range r {
		baseValue, ok := b[k]
		if !ok {
			fmt.Printf("%q missing in the baseline\n", k)
			continue
		}

		ratio := v.Seconds() / baseValue.Seconds()
		if ratio > (*threshold+1) && v > minDuration {
			fmt.Printf("WARNING: %q took %.2f more time than baseline (baseline: %v, result: %v)\n", k, ratio, baseValue, v)
			bad++
		} else {
			fmt.Printf("OK: %q\n", k)
			good++
		}
	}
	fmt.Printf("good: %d, bad %d\n", good, bad)
}

func compare(result *APIResponsiveness) error {
	baseline, err := parseResults(*baseline)
	if err != nil {
		return err
	}
	compareResults(baseline, result)
	return nil
}

func printSorted(result *APIResponsiveness) {
	sort.Slice(result.DataItems, func(i, j int) bool {
		return result.DataItems[i].Labels.count() > result.DataItems[j].Labels.count()
	})
	for _, i := range result.DataItems {
		fmt.Printf("%d %s\n", i.Labels.count(), i.Labels.asKey())
	}
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalf("expected 1 positional arguments: path to result, got: %v", flag.Args())
	}

	result, err := parseResults(flag.Arg(0))
	if err != nil {
		log.Fatalf("error while parsing result: %v", err)

	}

	switch *mode {
	case "compare":
		err = compare(result)
	case "sort":
		printSorted(result)
	default:
		err = errors.New("unknown mode")
	}
	if err != nil {
		log.Fatalf("failed: %v", err)
	}

}
