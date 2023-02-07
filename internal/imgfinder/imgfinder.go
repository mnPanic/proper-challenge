package imgfinder

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// A Scrapper knows how to obtain different things from webpages by GETting
// their content and parsing their HTML.
type Scrapper interface {
	CollectImageURLsFrom(page string) ([]string, error)
}

// An HTTPGetter knows how to perform HTTP GET requests
type HTTPGetter interface {
	Get(url string) (resp *http.Response, err error)
}

// A FileSystem provides access to the file system
type FileSystem interface {
	WriteFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(name string, perm os.FileMode) error
}

type Finder struct {
	scrapper   Scrapper
	fileSystem FileSystem
	getter     HTTPGetter
}

func New(scrapper Scrapper, fileSystem FileSystem, getter HTTPGetter) Finder {
	return Finder{
		scrapper:   scrapper,
		fileSystem: fileSystem,
		getter:     getter,
	}
}

func (f Finder) CollectAndDownloadImages(amount int, threads int, imagesDirectory string) error {
	imageURLs, err := f.collectImageURLs(amount)
	if err != nil {
		return err
	}

	fmt.Println("Downloading images")

	err = f.downloadImages(imageURLs[0:amount], imagesDirectory, threads)
	if err != nil {
		return fmt.Errorf("downloading images: %s", err)
	}

	return nil
}

func (f Finder) collectImageURLs(amount int) ([]string, error) {
	// Images are duplicated because they appear in the "Hot today" section and
	// on the homepage. Because we don't want to download them twice, we remove
	// the duplicates. We know the URLs will be the same because we converted
	// all of them to full size

	seenImageURLs := map[string]bool{}
	var imageURLs []string

	currentPage := 1
	for len(imageURLs) < amount {
		urls, err := f.scrapper.CollectImageURLsFrom(cheezburgerURLForPage(currentPage))
		if err != nil {
			return nil, fmt.Errorf("collecting image urls: %s", err)
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

func (f Finder) downloadImages(urls []string, basePath string, threads int) error {
	err := f.fileSystem.MkdirAll(basePath, 0777)
	if err != nil {
		return fmt.Errorf("creating destination directory %s: %s", basePath, err)
	}

	// Make a buffered channel so we can schedule all the jobs without blocking
	numJobs := len(urls)
	imagesToDownload := make(chan imageRequest, numJobs)
	results := make(chan error, numJobs)

	for w := 0; w < threads; w++ {
		go f.imageDownloadWorker(imagesToDownload, results)
	}

	for i, url := range urls {
		// i+1 to number from 1 and not 0
		path := filepath.Join(basePath, strconv.Itoa(i+1))
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

func (f Finder) imageDownloadWorker(imagesToDownload chan imageRequest, results chan error) {
	for image := range imagesToDownload {
		err := f.downloadImage(image.url, image.path)
		var result error
		if err != nil {
			result = fmt.Errorf("downloading image %s: %s", image.url, err)
		}

		results <- result
	}
}

func (f Finder) downloadImage(url string, filename string) error {
	resp, err := f.getter.Get(url)
	if err != nil {
		return fmt.Errorf("get: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code '%d' expected 200 OK", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading body: %s", err)
	}

	ext, err := detectFileExtension(resp.Header.Get("Content-Type"))
	if err != nil {
		return err
	}

	// Permissions don't matter much here
	err = f.fileSystem.WriteFile(filename+ext, body, 0777)
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
