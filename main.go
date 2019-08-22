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
// The story ID is mapped to the story.
var dal map[int]cachedStory

type cachedStory struct {
	time int64
}

func initDAL() {
	// Initialize the DAL.
	dal = map[int]cachedStory{}
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

		// Expiring items in the cache:
		// We can safely assume that items older than 5 days will no longer exist in the top stories list,
		// so delete the from the cache.

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
			// Otherwise, add it to the cache and the list of items to notify the user of.
			dal[item.ID] = cachedStory{time: item.Time}
			notifItems = append(notifItems, item)
			continue
		}

		// If our story does meet the treshold, but the time since the story was posted is greater than 2 days, add it to the cache anyway.
		if t := time.Unix(item.Time, 0); time.Since(t) >= maxAge {
			logrus.Debugf("story %d is older than two days: %v\n", item.ID, t)
			dal[item.ID] = cachedStory{time: item.Time}
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
	logrus.SetLevel(logrus.DebugLevel)
}

// bgDeleteOldStories deletes stories older than the given maxAge at the interval specified by frequency.
// TODO: Test this.
func bgDeleteOldStories(maxAge, frequency time.Duration) {
	for itemID, item := range dal {
		// TODO: Create and use a domain model for the item where the time is a time.Time, not int64.
		// If the story is 5 days or older, delete it from the cache to conserve space.
		if t := time.Unix(item.time, 0); time.Since(t) >= maxAge {
			logrus.WithFields(logrus.Fields{"item_id": itemID, "item_age": time.Since(t)}).Debug("Deleting old item from cache.")
			delete(dal, itemID)
		}
	}
	time.Sleep(frequency)
}

// notify lets the user know of the given stories...
// TODO: email, rss, slack, push notif?
func notify(items []apimodel.GetItemResponse) {
	for idx, item := range items {
		fmt.Printf("[%d] You have mail! --- %s\n", idx, item.Title)
		// TODO: Add URL?
	}
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
	maxCacheAge := time.Hour * 24 * 5
	cacheDeleteFrequency := time.Hour * 8
	newStoryCheckFrequency := time.Second * 10

	// Start deleting items in the background.
	go bgDeleteOldStories(maxCacheAge, cacheDeleteFrequency)

	// Start checking for stories.
	for {
		logrus.Debug("Checking for new stories...")
		start := time.Now()
		items, err := checkStories(scoreThreshold, maxAge, topStories)
		logrus.Debugf("it took %v to complete the run\n", time.Since(start))
		logrus.Debug("... Done checking for new stories.")
		if err != nil {
			logrus.Fatalf("error checking stories: %s", err)
		}

		notify(items)

		time.Sleep(newStoryCheckFrequency)
	}

	// TODO: Write a test that verifies the cache logic.
}
