package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/dsoprea/go-exif/v3"
	jpeg "github.com/dsoprea/go-jpeg-image-structure/v2"
	"golang.org/x/net/html"
)

type StringSet = map[string]struct{}

const (
	defaultLinksFileName        = "links.txt"
	defaultImageDirectoryName   = "album_arts"
	youTubeMusicAlbumCodeLength = 11
)

func writeFileWithExif(imageData []byte, filename string, albumCode string) {
	jpegMediaParser := jpeg.NewJpegMediaParser()
	intfc, err := jpegMediaParser.ParseBytes(imageData)
	if err != nil {
		log.Fatal(err)
	}

	segmentList := intfc.(*jpeg.SegmentList)

	exifBuilder, err := segmentList.ConstructExifBuilder()
	if err != nil {
		log.Fatal(err)
	}

	imageFileDirectory0Builder, err := exif.GetOrCreateIbFromRootIb(exifBuilder, "IFD0")
	if err != nil {
		log.Fatal(err)
	}

	err = imageFileDirectory0Builder.SetStandardWithName("ImageDescription", albumCode)
	if err != nil {
		log.Fatal(err)
	}

	err = segmentList.SetExif(exifBuilder)
	if err != nil {
		log.Fatal(err)
	}

	var updatedImageData bytes.Buffer
	err = segmentList.Write(&updatedImageData)
	if err != nil {
		log.Fatal(err)
	}

	// Octal value 0o0644 corresponds to unix file mode -rw-r--r--
	err = os.WriteFile(filename, updatedImageData.Bytes(), 0o0644)
	if err != nil {
		log.Fatal(err)
	}
}

func downloadAlbumArt(
	albumArtLink string,
	albumTitle string,
	albumCode string,
	waitGroup *sync.WaitGroup,
	albumArtClient *http.Client,
	albumArtDirectoryName string,
) {
	defer waitGroup.Done()

	response, err := albumArtClient.Get(albumArtLink)
	if err != nil {
		log.Printf("Error making http request: %s\n", err)
	}
	defer response.Body.Close()

	albumArt, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading request body %s\n", err)
	}

	albumTitle = strings.ReplaceAll(albumTitle, "/", " ")
	albumArtFileName := fmt.Sprintf("%s/%s.jpg", albumArtDirectoryName, albumTitle)

	if existingAlbumArtHasCode(albumArtFileName, albumCode) {
		log.Printf("Skipping %s\n", albumTitle)
		return
	}

	writeFileWithExif(albumArt, albumArtFileName, albumCode)
	log.Printf("Downloaded %s\n", albumTitle)
}

func existingAlbumArtHasCode(albumArtFileName string, albumCode string) bool {
	jpegMediaParser := jpeg.NewJpegMediaParser()
	albumArtFile, err := jpegMediaParser.ParseFile(albumArtFileName)
	if err != nil {
		return false
	}

	rootIfd, _, err := albumArtFile.Exif()
	if err != nil {
		return false
	}

	imageDescription, err := rootIfd.FindTagWithName("ImageDescription")
	if err != nil || len(imageDescription) != 1 {
		return false
	}

	value, err := imageDescription[0].Value()
	if err != nil {
		return false
	}

	switch existingCode := value.(type) {
	case string:
		return existingCode == albumCode
	case []byte:
		return string(existingCode) == albumCode
	}

	return false
}

func getNodeAttr(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func parseAlbumPage(node *html.Node, albumData map[string]string) {
	if node.Type == html.ElementNode && node.Data == "meta" {
		propertyValue := getNodeAttr(node, "property")

		if propertyValue == "og:image" || propertyValue == "og:title" {
			albumData[propertyValue] = getNodeAttr(node, "content")

			if len(albumData) == 2 {
				return
			}
		}
	}

	// traverse the child nodes
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		parseAlbumPage(child, albumData)
	}
}

func fetchAlbumPage(albumPageLink string, waitGroup *sync.WaitGroup, albumPageClient *http.Client, albumArtClient *http.Client, albumArtDirectoryName string) {
	defer waitGroup.Done()

	response, err := albumPageClient.Get(albumPageLink)
	if err != nil {
		log.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	document, err := html.Parse(response.Body)
	if err != nil {
		log.Printf("error parsing html: %s\n", err)
		os.Exit(1)
	}

	albumData := map[string]string{}
	parseAlbumPage(document, albumData)

	if len(albumData) != 2 {
		log.Printf("Album page didn't contain necessary data! %s\n", albumPageLink)
		return
	}

	if len(albumPageLink) < youTubeMusicAlbumCodeLength {
		log.Printf("Album page link didn't contain album code! %s\n", albumPageLink)
		return
	}

	albumCode := albumPageLink[len(albumPageLink)-youTubeMusicAlbumCodeLength:]
	waitGroup.Add(1)
	go downloadAlbumArt(
		albumData["og:image"],
		albumData["og:title"],
		albumCode,
		waitGroup,
		albumArtClient,
		albumArtDirectoryName,
	)
}

func getAllLinks(filename string) []string {
	linksFile, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	allLinks := string(linksFile)
	allLinks = strings.TrimSuffix(allLinks, "\n")

	return strings.Split(allLinks, ", ")
}

func getCodesOfExistingAlbumArt(albumArtDirectoryName string) StringSet {
	albumArtDirectory, err := os.ReadDir(albumArtDirectoryName)
	if err != nil {
		log.Println(err)
		err := os.MkdirAll(albumArtDirectoryName, 0o755)
		if err != nil {
			log.Fatal(err)
		}

		return make(StringSet)
	}

	jpegMediaParser := jpeg.NewJpegMediaParser()

	set := make(StringSet)

	for _, albumArtFileEntry := range albumArtDirectory {
		if albumArtFileEntry.IsDir() {
			continue
		}

		albumArtFileName := albumArtDirectoryName + "/" + albumArtFileEntry.Name()

		albumArtFile, err := jpegMediaParser.ParseFile(albumArtFileName)
		if err != nil {
			log.Print(err)
			continue
		}

		rootIfd, _, err := albumArtFile.Exif()
		if err != nil {
			log.Print(err)
			continue
		}

		imageDescription, err := rootIfd.FindTagWithName("ImageDescription")
		if err != nil || len(imageDescription) != 1 {
			continue
		}

		value, err := imageDescription[0].Value()
		if err != nil {
			continue
		}

		switch albumCode := value.(type) {
		case string:
			set[albumCode] = struct{}{}
		case []byte:
			set[string(albumCode)] = struct{}{}
		}
	}
	return set
}

func getLinksToDownload(linksFileName string, albumArtDirectoryName string) []string {
	allLinks := getAllLinks(linksFileName)
	existingAlbumCodes := getCodesOfExistingAlbumArt(albumArtDirectoryName)

	var linksToDownload []string

	for _, link := range allLinks {
		albumCode := link[len(link)-youTubeMusicAlbumCodeLength:]
		_, ok := existingAlbumCodes[albumCode]
		if !ok {
			linksToDownload = append(linksToDownload, link)
		}
	}

	return linksToDownload
}

func main() {
	var linksFileName string
	var imageDirectoryName string
	var shortLinksFileName string
	var shortImageDirectoryName string

	linksFileName = defaultLinksFileName
	imageDirectoryName = defaultImageDirectoryName

	flag.StringVar(&linksFileName, "links-file", defaultLinksFileName, "The file to read album links from.")
	flag.StringVar(&shortLinksFileName, "l", "", "Alias for --links-file.")
	flag.StringVar(&imageDirectoryName, "image-directory", defaultImageDirectoryName, "The directory to download album art images to.")
	flag.StringVar(&shortImageDirectoryName, "d", "", "Alias for --image-directory.")
	flag.Parse()

	if shortLinksFileName != "" {
		linksFileName = shortLinksFileName
	}
	if shortImageDirectoryName != "" {
		imageDirectoryName = shortImageDirectoryName
	}

	linksToDownload := getLinksToDownload(linksFileName, imageDirectoryName)

	youTubeMusicTransport := &http.Transport{}
	youTubeMusicClient := &http.Client{Transport: youTubeMusicTransport}
	defer youTubeMusicClient.CloseIdleConnections()

	googleUserContentTransport := &http.Transport{}
	googleUserContentClient := &http.Client{Transport: googleUserContentTransport}
	defer googleUserContentClient.CloseIdleConnections()

	var waitGroup sync.WaitGroup

	waitGroup.Add(len(linksToDownload))
	for _, link := range linksToDownload {
		go fetchAlbumPage(
			link,
			&waitGroup,
			youTubeMusicClient,
			googleUserContentClient,
			imageDirectoryName,
		)
	}

	waitGroup.Wait()
}
