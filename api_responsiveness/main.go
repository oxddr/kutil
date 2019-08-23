package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"
)

const (
	epsilon     = 0.00001
	minDuration = 50 * time.Millisecond
)

var (
	threshold = flag.Float64("threshold", 0.1, "Failure threshold.")
	mode      = flag.String("mode", "compare", "Mode")
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
}

func (l *Labels) asKey() string {
	key := fmt.Sprintf("%s %s/%s", l.Verb, l.Scope, l.Resource)
	if l.Subresource != "" {
		key = fmt.Sprintf("%s/%s", key, l.Subresource)
	}
	return key

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

func compare() error {
	if flag.NArg() != 2 {
		return errors.New("expected 2 positional arguments: path to baseline and result")
	}

	path := flag.Args()[0]
	baseline, err := parseResults(path)
	if err != nil {
		return err
	}
	path = flag.Args()[1]
	result, err := parseResults(path)
	if err != nil {
		return err

	}
	compareResults(baseline, result)
	return nil
}

func main() {
	flag.Parse()
	var err error
	switch *mode {
	case "compare":
		err = compare()
	default:
		err = errors.New("unknown mode")
	}
	if err != nil {
		log.Fatalf("failed: %v", err)
	}

}
