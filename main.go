package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/hbd/hnotif/apimodel"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// TODO: Think about extracting these into a go pkg for HN.

const (
	hnBaseURL       = "https://hacker-news.firebaseio.com/v0"
	hnTopStories    = "/topstories.json"
	hnTopStoriesURL = hnBaseURL + hnTopStories
	hnItem          = "/item/%s.json"
	hnItemURL       = hnBaseURL + hnItem
)

// dal is an in-memory database for caching stories.
// The story ID is mapped to the story score.
var dal map[int]struct{}

func initDAL() {
	// Initialize the DAL.
	dal = map[int]struct{}{}
}

func getTopStories() ([]int, error) {
	// Get list of top stories.
	resp, err := http.Get(hnTopStoriesURL)
	if err != nil {
		return nil, errors.Wrap(err, "http.Get top stories")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned unexpected status code: %d", hnTopStoriesURL, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadAll")
	}

	var topStoriesResp apimodel.TopStoriesResponse
	json.Unmarshal(body, &topStoriesResp)
	return []int(topStoriesResp), nil
}

func getItem(itemID int) (apimodel.GetItemResponse, error) {
	var item apimodel.GetItemResponse

	// Get the given item.
	resp, err := http.Get(fmt.Sprintf(hnItemURL, strconv.Itoa(itemID)))
	if err != nil {
		return item, errors.Wrap(err, "http.Get item")
	}
	if resp.StatusCode != http.StatusOK {
		return item, fmt.Errorf("%s returned unexpected status code: %d", hnItemURL, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return item, errors.Wrap(err, "ioutil.ReadAll")
	}

	json.Unmarshal(body, &item)
	return item, nil
}

func checkStories(scoreThreshold int, maxAge time.Duration, topStories []int) ([]apimodel.GetItemResponse, error) {
	notifItems := []apimodel.GetItemResponse{}
	numStoriesOverThreshold := 0

	for idx := 0; idx < len(topStories); idx++ {
		// Caching logic:
		// If a story is old enough, stop checking for it.
		// -> If a story is old enough, add it to the cache.
		// --> When a story is in the cache, we skip checking for it.
		// Note: We don't cache stories that are younger than 2 days old and have less than the threshold.

		// Skip stories that we've saved in the DAL.
		if _, ok := dal[topStories[idx]]; ok {
			logrus.Debugf("we've seen item %d before!\n", topStories[idx])
			continue
		} else {
			logrus.Debugf("we've not seen item %d before XX\n", topStories[idx])
		}

		// TODO: Use goroutines to do concurrently.
		item, err := getItem(topStories[idx])
		if err != nil {
			logrus.Fatalf("error getting item: %s", err)
		}

		// If our story meets the threshold and isn't in the cache, then add it to the notif list and the cache.
		if item.Score >= scoreThreshold {
			numStoriesOverThreshold++
			// If story is already in the cache, skip it.
			if _, ok := dal[topStories[idx]]; ok {
				continue
			}
			dal[item.ID] = struct{}{}
			notifItems = append(notifItems, item)
		}

		// If our story does meet the treshold, but the time since the story was posted is greater than 2 days, add it to the cache anyway.
		if t := time.Unix(item.Time, 0); time.Since(t) >= maxAge {
			logrus.Debugf("story %d is older than two days: %v\n", item.ID, t)
			dal[item.ID] = struct{}{}
		} else { // If neither are true, then just don't do anything.
			logrus.Debugf("story %d is NOT older than two days: %v\n", item.ID, t)
		}
	}

	return notifItems, nil
}

func setupLogger() {
	// TODO: Use config to configure logger.
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stderr)
	logrus.SetLevel(logrus.InfoLevel)
}

func main() {
	initDAL()
	setupLogger()

	topStories, err := getTopStories()
	if err != nil {
		logrus.Fatalf("error getting top stories: %s", err)
	}
	if len(topStories) < 500 {
		logrus.Fatalf("expected 500 top stories but got %d", len(topStories))
	}

	// Find all items in the list that meet the threshold.
	scoreThreshold := 100        // TODO: Default. Let the user set this.
	maxAge := time.Hour * 24 * 2 // TODO: Default. Let the user set this.

	println("starting 1st run")
	start := time.Now()
	items, err := checkStories(scoreThreshold, maxAge, topStories)
	fmt.Printf("it took %v to complete the run\n", time.Since(start))
	println("end 1st run")
	if err != nil {
		logrus.Fatalf("error checking stories: %s", err)
	}
	for idx, item := range items {
		fmt.Printf("[%d] You have mail! --- %s\n", idx, item.Title)
	}

	// Write a test that replicates this manual test.
	// We expect not too see any results when this is run one after another.
	println("starting 2nd run")
	start = time.Now()
	items, err = checkStories(scoreThreshold, maxAge, topStories)
	fmt.Printf("it took %v to complete the run\n", time.Since(start))
	println("end 2nd run")
	if err != nil {
		logrus.Fatalf("error checking stories: %s", err)
	}
	for idx, item := range items {
		fmt.Printf("[%d] You have mail! --- %s\n", idx, item.Title)
	}
}
