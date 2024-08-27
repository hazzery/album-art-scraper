package main

import (
	"fmt"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

func getNodeAttr(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func processMetaNode(node *html.Node) {
	var titleNode *html.Node
	var imageLinkNode *html.Node
	var propertyValue = getNodeAttr(node, "property")

	if propertyValue == "og:image" {
		imageLinkNode = node
	} else if propertyValue == "og:title" {
		titleNode = node
	}
	if titleNode == nil || imageLinkNode == nil {
		return
	}

	for _, attribute := range titleNode.Attr {
		if attribute.Key == "content" {

		}
	}
}

func parseAlbumPage(node *html.Node) {
	if node.Type == html.ElementNode && node.Data == "meta" {
		processMetaNode(node)
	}

	// traverse the child nodes
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		parseAlbumPage(child)
	}
}

func fetchAlbumPage(albumPageLink string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	response, err := http.Get(albumPageLink)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	//body, err := io.ReadAll(response.Body)
	//fmt.Println(string(body))

	document, err := html.Parse(response.Body)
	if err != nil {
		fmt.Printf("error parsing html: %s\n", err)
		os.Exit(1)
	}
	parseAlbumPage(document)
}

func main() {
	linksFile, err := os.ReadFile("links.txt")
	if err != nil {
		log.Fatal(err)
	}
	allLinks := strings.TrimSuffix(string(linksFile), "\n")
	links := strings.Split(allLinks, ", ")

	var waitGroup sync.WaitGroup

	waitGroup.Add(1)
	go fetchAlbumPage(links[0], &waitGroup)

	//waitGroup.Add(len(links))
	//for _, link := range links {
	//	go fetchAlbumPage(link, &waitGroup)
	//}

	waitGroup.Wait()
}
