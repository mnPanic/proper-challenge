package imgfinder_test

import (
	"cat-scraper/internal/imgfinder"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadsFoundImages(t *testing.T) {
	const url = "https://i.chzbgr.com/full/9730332160/h6860EF7A"
	content := []byte("hello")

	scrapper := MockScrapper{
		URLsByPage: map[string][]string{
			"https://icanhas.cheezburger.com/": {url},
		},
	}

	getter := StaticGetter{
		ResponseByURL: map[string]Response{
			url: {Content: content, ContentType: "image/jpeg", StatusCode: http.StatusOK},
		},
	}
	writer := &MockFileWriter{}

	finder := imgfinder.New(scrapper, writer, getter)

	err := finder.CollectAndDownloadImages(1, 1, "images/")
	require.NoError(t, err)

	writer.AssertCreatedDirectories(t, "images/")
	writer.AssertWroteFiles(t,
		file{Content: content, Name: "images/1.jpg"},
	)
}

func TestGoesToTheNextPageAndPicksExtension(t *testing.T) {
	const url = "https://i.chzbgr.com/full/9730332160/h6860EF7A"
	content := []byte("hello")

	const secondURL = "https://i.chzbgr.com/full/2/h6860EF7A"
	secondContent := []byte("bye")

	const thirdURL = "https://i.chzbgr.com/full/3/h6860EF7A"
	thirdContent := []byte("welcome back")

	scrapper := MockScrapper{
		URLsByPage: map[string][]string{
			"https://icanhas.cheezburger.com/":       {url},
			"https://icanhas.cheezburger.com/page/2": {secondURL},
			"https://icanhas.cheezburger.com/page/3": {thirdURL},
		},
	}

	getter := StaticGetter{
		ResponseByURL: map[string]Response{
			url:       {Content: content, ContentType: "image/jpeg", StatusCode: http.StatusOK},
			secondURL: {Content: secondContent, ContentType: "image/png", StatusCode: http.StatusOK},
			thirdURL:  {Content: thirdContent, ContentType: "image/gif", StatusCode: http.StatusOK},
		},
	}

	writer := &MockFileWriter{}

	finder := imgfinder.New(scrapper, writer, getter)

	err := finder.CollectAndDownloadImages(3, 3, "images/")
	require.NoError(t, err)

	// It writes the content obtained from the URLs, picking the correct
	// extension based on the content type.
	writer.AssertCreatedDirectories(t, "images/")
	writer.AssertWroteFiles(t,
		file{Content: content, Name: "images/1.jpg"},
		file{Content: secondContent, Name: "images/2.png"},
		file{Content: thirdContent, Name: "images/3.gif"},
	)
}

func TestIgnoresDuplicatedURLs(t *testing.T) {
	const url = "https://i.chzbgr.com/full/9730332160/h6860EF7A"
	content := []byte("hello")

	const secondURL = "https://i.chzbgr.com/full/2/h6860EF7A"
	secondContent := []byte("bye")

	scrapper := MockScrapper{
		URLsByPage: map[string][]string{
			"https://icanhas.cheezburger.com/":       {url, url},       // ignores in same page
			"https://icanhas.cheezburger.com/page/2": {secondURL, url}, // ignores in different page
		},
	}

	getter := StaticGetter{
		ResponseByURL: map[string]Response{
			url:       {Content: content, ContentType: "image/jpeg", StatusCode: http.StatusOK},
			secondURL: {Content: secondContent, ContentType: "image/png", StatusCode: http.StatusOK},
		},
	}

	writer := &MockFileWriter{}

	finder := imgfinder.New(scrapper, writer, getter)

	err := finder.CollectAndDownloadImages(2, 3, "images/")
	require.NoError(t, err)

	// Despite getting 4 URLs from the scrapper, it only downloads and saves 2
	// (the unique ones)
	writer.AssertCreatedDirectories(t, "images/")
	writer.AssertWroteFiles(t,
		file{Content: content, Name: "images/1.jpg"},
		file{Content: secondContent, Name: "images/2.png"},
	)
}

func TestScrapError(t *testing.T) {
	scrapper := MockScrapper{
		Error: errors.New("failed"),
	}

	getter := StaticGetter{}
	writer := &MockFileWriter{}

	finder := imgfinder.New(scrapper, writer, getter)

	err := finder.CollectAndDownloadImages(1, 3, "images/")
	require.EqualError(t, err, "collecting image urls: failed")
}

func TestCreateDirectoryError(t *testing.T) {
	const url = "https://i.chzbgr.com/full/9730332160/h6860EF7A"
	content := []byte("hello")

	scrapper := MockScrapper{
		URLsByPage: map[string][]string{
			"https://icanhas.cheezburger.com/": {url},
		},
	}

	getter := StaticGetter{
		ResponseByURL: map[string]Response{
			url: {Content: content, ContentType: "image/jpeg", StatusCode: http.StatusOK},
		},
	}
	writer := &MockFileWriter{
		MkdirErr: errors.New("failed"),
	}

	finder := imgfinder.New(scrapper, writer, getter)

	err := finder.CollectAndDownloadImages(1, 3, "images/")
	require.EqualError(t, err, "downloading images: creating destination directory images/: failed")
}

func TestSaveFileError(t *testing.T) {
	const url = "https://i.chzbgr.com/full/9730332160/h6860EF7A"
	content := []byte("hello")

	scrapper := MockScrapper{
		URLsByPage: map[string][]string{
			"https://icanhas.cheezburger.com/": {url},
		},
	}

	getter := StaticGetter{
		ResponseByURL: map[string]Response{
			url: {Content: content, ContentType: "image/jpeg", StatusCode: http.StatusOK},
		},
	}
	writer := &MockFileWriter{
		WriteErr: errors.New("failed"),
	}

	finder := imgfinder.New(scrapper, writer, getter)

	err := finder.CollectAndDownloadImages(1, 3, "images/")
	require.EqualError(t, err, "downloading images: downloading image https://i.chzbgr.com/full/9730332160/h6860EF7A: saving: failed")
}

func TestImageDownloadUnexpectedContentType(t *testing.T) {
	const url = "https://i.chzbgr.com/full/9730332160/h6860EF7A"
	content := []byte("hello")

	scrapper := MockScrapper{
		URLsByPage: map[string][]string{
			"https://icanhas.cheezburger.com/": {url},
		},
	}

	getter := StaticGetter{
		ResponseByURL: map[string]Response{
			url: {Content: content, ContentType: "invalid content type", StatusCode: http.StatusOK},
		},
	}

	writer := &MockFileWriter{}

	finder := imgfinder.New(scrapper, writer, getter)

	err := finder.CollectAndDownloadImages(1, 3, "images/")
	require.EqualError(t, err, "downloading images: downloading image https://i.chzbgr.com/full/9730332160/h6860EF7A: unexpected content type 'invalid content type'")
}

func TestImageDownloadUnexpectedStatusCode(t *testing.T) {
	const url = "https://i.chzbgr.com/full/9730332160/h6860EF7A"
	content := []byte("hello")

	scrapper := MockScrapper{
		URLsByPage: map[string][]string{
			"https://icanhas.cheezburger.com/": {url},
		},
	}

	getter := StaticGetter{
		ResponseByURL: map[string]Response{
			url: {Content: content, ContentType: "image/png", StatusCode: http.StatusInternalServerError},
		},
	}

	writer := &MockFileWriter{}

	finder := imgfinder.New(scrapper, writer, getter)

	err := finder.CollectAndDownloadImages(1, 3, "images/")
	require.EqualError(t, err, "downloading images: downloading image https://i.chzbgr.com/full/9730332160/h6860EF7A: unexpected status code '500' expected 200 OK")
}

type MockFileWriter struct {
	writtenFiles []file
	WriteErr     error

	createdDirectories []string
	MkdirErr           error
}

type file struct {
	Name    string
	Content []byte
}

func (m *MockFileWriter) WriteFile(name string, data []byte, _ os.FileMode) error {
	if m.WriteErr != nil {
		return m.WriteErr
	}

	m.writtenFiles = append(m.writtenFiles, file{Name: name, Content: data})
	return nil
}

func (m *MockFileWriter) MkdirAll(name string, _ os.FileMode) error {
	if m.MkdirErr != nil {
		return m.MkdirErr
	}

	m.createdDirectories = append(m.createdDirectories, name)
	return nil
}

func (m *MockFileWriter) AssertWroteFiles(t *testing.T, expectedFiles ...file) {
	// Check the elements match but not the order, because when executed with
	// threads they may be written in a different order than obtained.
	assert.ElementsMatch(t, expectedFiles, m.writtenFiles)
}

func (m *MockFileWriter) AssertCreatedDirectories(t *testing.T, expectedDirectories ...string) {
	assert.ElementsMatch(t, expectedDirectories, m.createdDirectories)
}

type MockScrapper struct {
	URLsByPage map[string][]string
	Error      error
}

func (s MockScrapper) CollectImageURLsFrom(pageURL string) ([]string, error) {
	if s.Error != nil {
		return nil, s.Error
	}

	urls, ok := s.URLsByPage[pageURL]
	if !ok {
		return nil, fmt.Errorf("url '%s' not found", pageURL)
	}

	return urls, nil
}

type StaticGetter struct {
	ResponseByURL map[string]Response

	Err error
}

type Response struct {
	Content     []byte
	ContentType string
	StatusCode  int
}

func (s StaticGetter) Get(url string) (*http.Response, error) {
	if s.Err != nil {
		return nil, s.Err
	}

	response, ok := s.ResponseByURL[url]
	if !ok {
		return nil, fmt.Errorf("url '%s' not found", url)
	}

	rec := httptest.NewRecorder()
	if response.Content != nil {
		rec.Write(response.Content)
	}

	resp := rec.Result()
	resp.Header.Set("Content-Type", response.ContentType)
	resp.StatusCode = response.StatusCode

	return resp, nil
}
