package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	exif "github.com/dsoprea/go-exif/v3"
	"golang.org/x/net/html"
)

type StringSet = map[string]struct{}

func downloadAlbumArt(
	albumArtLink string,
	albumTitle string,
	waitGroup *sync.WaitGroup,
	albumArtClient *http.Client,
	albumArtDirectoryName string,
) {
	defer waitGroup.Done()

	response, err := albumArtClient.Get(albumArtLink)
	if err != nil {
		fmt.Printf("Error making http request: %s\n", err)
	}
	defer response.Body.Close()

	albumArt, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("Error reading request body %s\n", err)
	}

	albumTitle = strings.ReplaceAll(albumTitle, "/", " ")
	albumArtFileName := fmt.Sprintf("%s/%s.jpg", albumArtDirectoryName, albumTitle)

	err = os.WriteFile(albumArtFileName, albumArt, 0666)
	if err != nil {
		fmt.Printf("Error writing file: %s\n", err)
	}

	fmt.Printf("Downloaded %s\n", albumTitle)
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

func fetchAlbumPage(albumPageLink string, waitGroup *sync.WaitGroup, albumPageClient *http.Client, albumArtClient *http.Client) {
	defer waitGroup.Done()

	response, err := albumPageClient.Get(albumPageLink)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	document, err := html.Parse(response.Body)
	if err != nil {
		fmt.Printf("error parsing html: %s\n", err)
		os.Exit(1)
	}

	albumData := map[string]string{}
	parseAlbumPage(document, albumData)

	if len(albumData) != 2 {
		fmt.Printf("Album page didn't contain necessary data! %s\n", albumPageLink)
		return
	}

	waitGroup.Add(1)
	go downloadAlbumArt(albumData["og:image"], albumData["og:title"], waitGroup, albumArtClient)
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
		os.Mkdir(albumArtDirectoryName, os.ModePerm)

		var set StringSet
		return set
	} else {
		var set StringSet

		for _, albumArtFileEntry := range albumArtDirectory {
			if !albumArtFileEntry.IsDir() {
				continue
			}

			albumArtFile, err := os.Open(albumArtDirectoryName + "/" + albumArtFileEntry.Name())
			if err != nil {
				log.Fatalf("Failure to read in existing file %s: %s", albumArtFileEntry.Name(), err)
			}
			metadata, err := exif.Decode(albumArtFile)
			if err != nil {
				log.Fatal(err)
			}
			albumCode, err := metadata.Get(exif.ImageUniqueID)
			if err != nil {
				log.Fatal(err)
			}

			set[albumCode.String()] = struct{}{}
		}
		return set
	}
}

func getLinksToDownload(linksFileName string, albumArtDirectoryName string) []string {
	allLinks := getAllLinks(linksFileName)
	existingAlbumCodes := getCodesOfExistingAlbumArt(albumArtDirectoryName)

	const YOUTUBE_MUSIC_ALBUM_CODE_LENGTH = 11

	var linksToDownload []string

	for _, link := range allLinks {
		albumCode := link[len(link)-YOUTUBE_MUSIC_ALBUM_CODE_LENGTH:]
		_, ok := existingAlbumCodes[albumCode]
		if !ok {
			linksToDownload = append(linksToDownload, link)
		}
	}

	return linksToDownload
}

func main() {
	linksToDownload := getLinksToDownload("links.txt", "album_arts")

	youTubeMusicTransport := &http.Transport{}
	youTubeMusicClient := &http.Client{Transport: youTubeMusicTransport}
	defer youTubeMusicClient.CloseIdleConnections()

	googleUserContentTransport := &http.Transport{}
	googleUserContentClient := &http.Client{Transport: googleUserContentTransport}
	defer googleUserContentClient.CloseIdleConnections()

	var waitGroup sync.WaitGroup

	waitGroup.Add(len(linksToDownload))
	for _, link := range linksToDownload {
		go fetchAlbumPage(link, &waitGroup, youTubeMusicClient, googleUserContentClient)
	}

	waitGroup.Wait()
}
