package main

import (
	"fmt"
	"sync"
)

type Fetcher interface {
	// Fetch returns the body of URL and
	// a slice of URLs found on that page.
	Fetch(url string) (body string, urls []string, err error)
}

// A place to cache the results of crawling pages
type crawlerCache struct {
	m map[string]fetchResult
	sync.Mutex
}

// Add a key, making sure nobody else is modifying the cache at the same time
func (c *crawlerCache) Add(key string, val fetchResult) {
	c.Lock()
	c.m[key] = val
	c.Unlock()
}

// Check if a key has been added, making sure nobody else is modifying the cache at the same time
func (c *crawlerCache) Contains(key string) bool {
	c.Lock()
	defer c.Unlock()
	_, found := c.m[key]
	return found
}

type fetchResult struct {
	body string
	urls []string
	err  error
}

var cache = &crawlerCache{m: make(map[string]fetchResult)}
var loading = fetchResult{} // dummy value to put in cache while crawling is in progress

// Crawl uses fetcher to recursively crawl
// pages starting with url, to a maximum of depth.
func Crawl(url string, depth int, fetcher Fetcher) {
	// have we already hit the max depth, or already seen this url?
	if depth <= 0 || cache.Contains(url) {
		return
	}

	cache.Add(url, loading)                      // "claim" this url in the cache while loading results, so we don't double fetch
	body, urls, err := fetcher.Fetch(url)        // actually fetch the results
	cache.Add(url, fetchResult{body, urls, err}) // overwrite loading value in the cache with the proper result

	if err != nil {
		return
	}

	done := make(chan bool)
	for _, childURL := range urls {
		// crawl child urls asynchronously
		go func(url string) {
			Crawl(url, depth-1, fetcher)
			// send a signal to the channel for each finished child crawl
			//  * note that THIS WILL BLOCK until something receives it
			//  * however, because it's run in a goroutine, it'll only block the goroutine
			done <- true
		}(childURL)
	}

	// don't exit Crawl until all child crawls are finished
	//  * each call of <-done blocks on receiving a value in the channel
	//  * we do it the exact right number of times, because we know exactly how many values the channel will receive
	for _ = range urls {
		<-done
	}
}

func main() {
	Crawl("http://golang.org/", 4, fetcher)

	for url, fetchResult := range cache.m {
		if fetchResult.err != nil {
			fmt.Printf("%v failed: %v\n", url, fetchResult.err)
		} else {
			fmt.Printf("%v was fetched. Body was: %s\n", url, fetchResult.body)
		}
	}
}

// fakeFetcher is Fetcher that returns canned results.
type fakeFetcher map[string]*fakeResult

type fakeResult struct {
	body string
	urls []string
}

func (f fakeFetcher) Fetch(url string) (string, []string, error) {
	if res, ok := f[url]; ok {
		return res.body, res.urls, nil
	}
	return "", nil, fmt.Errorf("not found: %s", url)
}

// fetcher is a populated fakeFetcher.
var fetcher = fakeFetcher{
	"http://golang.org/": &fakeResult{
		"The Go Programming Language",
		[]string{
			"http://golang.org/pkg/",
			"http://golang.org/cmd/",
		},
	},
	"http://golang.org/pkg/": &fakeResult{
		"Packages",
		[]string{
			"http://golang.org/",
			"http://golang.org/cmd/",
			"http://golang.org/pkg/fmt/",
			"http://golang.org/pkg/os/",
		},
	},
	"http://golang.org/pkg/fmt/": &fakeResult{
		"Package fmt",
		[]string{
			"http://golang.org/",
			"http://golang.org/pkg/",
		},
	},
	"http://golang.org/pkg/os/": &fakeResult{
		"Package os",
		[]string{
			"http://golang.org/",
			"http://golang.org/pkg/",
		},
	},
}
