package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

// Command line flags
var (
	amount  = flag.Int("amount", 10, "how many memes to download")
	threads = flag.Int("threads", 1, "number of threads that will download images concurrently (max: 5)")
)

func main() {
	flag.Parse()
	fmt.Printf("Downloading %d memes with %d threads\n", *amount, *threads)
	err := collectAndDownloadImages(*amount, *threads)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Images saved successfully")
}

func collectAndDownloadImages(amount int, threads int) error {
	imageURLs, err := collectImageURLs(amount)
	if err != nil {
		return err
	}

	fmt.Println("Downloading images")

	const imagesDirectory = "images/"
	err = downloadImages(imageURLs[0:amount], imagesDirectory, threads)
	if err != nil {
		return fmt.Errorf("downloading images: %s", err)
	}

	return nil
}

func collectImageURLs(amount int) ([]string, error) {
	// Images are duplicated because they appear in the "Hot today" section and
	// on the homepage. Because we don't want to download them twice, we remove
	// the duplicates. We know the URLs will be the same because we converted
	// all of them to full size

	seenImageURLs := map[string]bool{}
	var imageURLs []string

	currentPage := 1
	for len(imageURLs) < amount {
		urls := collectImageURLsFrom(cheezburgerURLForPage(currentPage))

		urls, err := convertToFullSizeURLs(urls)
		if err != nil {
			return nil, fmt.Errorf("converting image urls: %s", err)
		}

		duplicates := 0
		for _, url := range urls {
			if _, seen := seenImageURLs[url]; !seen {
				seenImageURLs[url] = true
				imageURLs = append(imageURLs, url)
			} else {
				duplicates++
			}
		}

		fmt.Printf("Found %d images (%d duplicates, %d new)\n", len(urls), duplicates, len(urls)-duplicates)

		currentPage++
	}

	return imageURLs, nil
}

func cheezburgerURLForPage(pageNumber int) string {
	if pageNumber == 1 {
		return "https://icanhas.cheezburger.com/"
	}

	return fmt.Sprintf("https://icanhas.cheezburger.com/page/%d", pageNumber)
}

type imageRequest struct {
	url  string
	path string
}

func downloadImages(urls []string, basePath string, threads int) error {
	err := os.MkdirAll(basePath, 0777)
	if err != nil {
		return fmt.Errorf("creating destination directory %s: %s", basePath, err)
	}

	// Make a buffered channel so we can schedule all the jobs without blocking
	numJobs := len(urls)
	imagesToDownload := make(chan imageRequest, numJobs)
	results := make(chan error, numJobs)

	for w := 0; w < threads; w++ {
		go imageDownloadWorker(imagesToDownload, results)
	}

	for i, url := range urls {
		// i+1 to number from 1 and not 0
		path := filepath.Join(basePath, strconv.Itoa(i+1))
		fmt.Printf("image #%d: %s\n", i+1, url)

		imagesToDownload <- imageRequest{url: url, path: path}
	}

	// Grab all results, check no download failed
	for r := 0; r < numJobs; r++ {
		err := <-results
		if err != nil {
			return err
		}
	}

	return nil
}

func imageDownloadWorker(imagesToDownload chan imageRequest, results chan error) {
	for image := range imagesToDownload {
		err := downloadImage(image.url, image.path)
		var result error
		if err != nil {
			result = fmt.Errorf("downloading image %s: %s", image.url, err)
		}

		results <- result
	}
}

func downloadImage(url string, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get: %s", err)
	}

	// TODO: Consider using io.Copy and opening the file manually to avoid
	// having the body twice in memory
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading body: %s", err)
	}

	ext, err := detectFileExtension(resp.Header.Get("Content-Type"))
	if err != nil {
		return err
	}

	// Permissions don't matter much here
	err = os.WriteFile(filename+ext, body, 0777)
	if err != nil {
		return fmt.Errorf("saving: %s", err)
	}

	return nil
}

func detectFileExtension(contentType string) (string, error) {
	contentTypeToExt := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
	}

	ext, ok := contentTypeToExt[contentType]
	if !ok {
		return "", fmt.Errorf("unexpected content type '%s'", contentType)
	}

	return ext, nil
}

func collectImageURLsFrom(pageURL string) []string {
	var imgURLs []string

	c := colly.NewCollector()

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	// Select all img elements and save their source
	c.OnHTML(`img`, func(e *colly.HTMLElement) {
		// Memes have classes resp-media (for content that loads with the page)
		// and resp-media lazyload (for lazy loaded content). The rest are not
		// memes.
		imgClass := e.Attr("class")
		if !(imgClass == "resp-media" || imgClass == "resp-media lazyload") {
			return
		}

		// Because the page lazy loads images and only loads them when they
		// appear on the viewport, they start out with the src in `data-src`
		// while `src` has a placeholder value (that's not an url starting
		// with https)
		imgURL := e.Attr("src")
		if !strings.HasPrefix(imgURL, "https") {
			imgURL = e.Attr("data-src")
		}

		//fmt.Printf("class: %s\n", e.Attr("class"))
		//fmt.Println("Image link found", imgURL)
		imgURLs = append(imgURLs, imgURL)
	})

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})

	c.Visit(pageURL)

	return imgURLs
}

func convertToFullSizeURLs(urls []string) ([]string, error) {
	var fullSizeURLs []string
	for _, url := range urls {
		fullSizeURL, err := getFullSizeVersion(url)
		if err != nil {
			return nil, fmt.Errorf("can't get full size version of '%s': %s", url, err)
		}

		fullSizeURLs = append(fullSizeURLs, fullSizeURL)
	}

	return fullSizeURLs, nil
}

func getFullSizeVersion(imageURL string) (string, error) {
	// Cheezburger image urls have the following format
	//
	//	https://i.chzbrg.com/{size}/{id1}/{id2}/{slug}
	//
	// We want the full size version and not a downscaled thumbnail. This will
	// also help prevent downloading the same images more than once.
	//
	// Some examples:
	// - https://i.chzbgr.com/full/9730332160/h6860EF7A/just-no
	// - https://i.chzbgr.com/thumb1200/19206661/h5E69E7B5/feral-trapped-last-week-he-has-strong-feelings-about-domestication-me-and-the-horse-i-rode-in-on
	//
	// The slug doesn't matter, and sometimes slugs are different (so we
	// download them differently). To avoid that, also remove the slug.

	url, err := url.Parse(imageURL)
	if err != nil {
		return "", fmt.Errorf("parse: %s", err)
	}

	// Trim the leading / and split by / to separate the size from the rest of
	// the path so we can replace it.
	parts := strings.Split(strings.TrimPrefix(url.Path, "/"), "/")
	if len(parts) != 4 {
		return "", errors.New("unexpected path format, expected {size}/{id1}/{id2}/{slug}")
	}

	// Replace size for full and remove the slug
	url.Path = fmt.Sprintf("full/%s/%s", parts[1], parts[2])

	return url.String(), nil
}
