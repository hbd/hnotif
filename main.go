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

func checkStories(scoreThreshold int, topStories []int) error {
	avgScore := 0
	topScore := 0
	botScore := 500
	numStoriesOverThreshold := 0
	oldestStoryTime := time.Second * 1
	items := make([]apimodel.GetItemResponse, 0, 100)
	for idx := 0; idx < len(topStories); idx++ {
		// Cache the stories.
		// If a story is old enough, stop checking for it.
		// -> If a story is old enough, add it to the cache.
		// --> When a story is in the cache, we skip checking for it.

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
		items = append(items, item)
		avgScore += item.Score
		if item.Score > topScore {
			topScore = item.Score
		}
		if item.Score < botScore {
			botScore = item.Score
		}
		if item.Score >= scoreThreshold {
			numStoriesOverThreshold++
		}
		if t := time.Unix(item.Time, 0); time.Since(t) > oldestStoryTime {
			oldestStoryTime = time.Since(t)
		}

		// If the time since the story was posted is greater than 2 days,
		// then add it to the cache.
		if t := time.Unix(item.Time, 0); time.Since(t) >= time.Hour*24*2 {
			logrus.Debugf("story %d is older than two days: %v\n", item.ID, t)
			dal[item.ID] = struct{}{}
		} else {
			logrus.Debugf("story %d is NOT older than two days: %v\n", item.ID, t)
		}
	}

	avgScore = avgScore / 100
	logrus.Debugf("avg: %d\ntop: %d\nbot: %d\nnumOverThreshold: %d\nstories:\n\t1: %d\n\t10: %d\n\t100: %d\n",
		avgScore,
		topScore,
		botScore,
		numStoriesOverThreshold,
		items[0].Score,
		items[9].Score,
		items[99].Score,
	)

	return nil
}

func setupLogger() {
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
	scoreThreshold := 100 // Arbitrary. Let the user set this.

	err = checkStories(scoreThreshold, topStories)
	if err != nil {
		logrus.Fatalf("error checking stories: %s", err)
	}
}
