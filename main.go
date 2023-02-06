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

var amount = flag.Int("amount", 10, "how many memes to download")

func main() {
	flag.Parse()
	fmt.Printf("Downloading %d memes\n", *amount)
	err := collectAndDownloadImages(*amount)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Images saved successfully")
}

func collectAndDownloadImages(amount int) error {
	var imageURLs []string
	currentPage := 1
	for len(imageURLs) < amount {
		urls := collectImageURLsFrom(cheezburgerURLForPage(currentPage))
		fmt.Printf("Found %d images\n", len(urls))

		urls, err := convertToFullSizeURLs(urls)
		if err != nil {
			return fmt.Errorf("converting image urls: %s", err)
		}

		imageURLs = append(imageURLs, urls...)

		prevLen := len(imageURLs)
		imageURLs = dedupURLs(imageURLs)
		fmt.Printf("Removed %d duplicates\n", prevLen-len(imageURLs))
		currentPage++
	}

	imagesDirectory := "images/"
	err := os.MkdirAll(imagesDirectory, 0777)
	if err != nil {
		return fmt.Errorf("creating destination directory %s: %s", imagesDirectory, err)
	}

	err = downloadImages(imageURLs[0:amount], imagesDirectory)
	if err != nil {
		return fmt.Errorf("downloading images: %s", err)
	}

	return nil
}

func cheezburgerURLForPage(pageNumber int) string {
	if pageNumber == 1 {
		return "https://icanhas.cheezburger.com/"
	}

	return fmt.Sprintf("https://icanhas.cheezburger.com/page/%d", pageNumber)
}

func downloadImages(urls []string, basePath string) error {
	for i, url := range urls {
		// i+1 to number from 1 and not 0
		path := filepath.Join(basePath, strconv.Itoa(i+1))

		err := downloadImage(url, path)
		if err != nil {
			return fmt.Errorf("downloading image #%d: %s", i, err)
		}
	}

	return nil
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
	// We want the full size version and not a downscaled thumbnail.
	//
	// Some examples:
	// - https://i.chzbgr.com/full/9730332160/h6860EF7A/just-no
	// -
	// https://i.chzbgr.com/thumb1200/19206661/h5E69E7B5/feral-trapped-last-week-he-has-strong-feelings-about-domestication-me-and-the-horse-i-rode-in-on
	url, err := url.Parse(imageURL)
	if err != nil {
		return "", fmt.Errorf("parse: %s", err)
	}

	// Trim the leading / and split by / to separate the size from the rest of
	// the path so we can replace it.
	parts := strings.SplitN(strings.TrimPrefix(url.Path, "/"), "/", 2)
	if len(parts) != 2 {
		return "", errors.New("unexpected path format")
	}

	// parts[0] is the size and parts[1] is the rest of the path
	url.Path = fmt.Sprintf("full/%s", parts[1])

	return url.String(), nil
}

func dedupURLs(urls []string) []string {
	// Images are duplicated because they appear in the "Hot today" section and
	// on the homepage. Because we don't want to download them twice, we remove
	// the duplicates. We know the URLs will be the same because we converted
	// all of them to full size

	allURLs := map[string]bool{}
	var uniqueURLs []string

	for _, url := range urls {
		if _, exists := allURLs[url]; !exists {
			allURLs[url] = true
			uniqueURLs = append(uniqueURLs, url)
		}
	}

	return uniqueURLs

}
