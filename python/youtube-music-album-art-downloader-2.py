import aiohttp
import asyncio
import sys
import os

from bs4 import BeautifulSoup
import piexif


def print_error(*args, **kwargs) -> None:
    print(*args, file=sys.stderr, **kwargs)


async def download_album_art(
    session: aiohttp.ClientSession, link: str, album_title: str, youtube_album_code: str
) -> None:
    """Asyncronously download the album art image from the specified link.

    :param session: The HTTP session to send the request with (reuses connection to improve speed).
    :param link: The URL to the image to download.
    :param album_title: The title of the album whose art to download.
    """
    async with session.get(link) as album_art_response:
        sanitised_album_title = album_title.replace("/", " ")
        filename = f"album_arts/{sanitised_album_title}.jpg"
        exif_data = piexif.dump(
            {
                "0th": {piexif.ImageIFD.ImageDescription: youtube_album_code.encode()},
                "Exif": {},
                "GPS": {},
                "1st": {},
                "thumbnail": None,
            }
        )
        piexif.insert(exif_data, await album_art_response.read(), filename)


async def request_album_page(
    session: aiohttp.ClientSession,
    album_art_session: aiohttp.ClientSession,
    link: str,
    download_tasks: list[asyncio.Task],
) -> None:
    """Asyncronously fetch the YouTube Music album page at the specified URL.

    :param session: The HTTP session to send the request with (reuses connection to improve speed).
    :param album_art_session: The HTTP session to send album art image request with (reuses connection to improve speed).
    :param link: The URL to the YouTube Music album page.
    :param download_tasks: A list of tasks to append the image download task to.
    """
    async with session.get(link) as album_page_response:
        parsed_document = BeautifulSoup(
            await album_page_response.text(), features="html.parser"
        )

        album_title_element = parsed_document.find("meta", attrs={"name": "title"})
        if album_title_element is None:
            print_error("No album title for", link)
            return

        album_title = album_title_element.attrs["content"]

        album_art_link_element = parsed_document.find(
            "meta", attrs={"property": "og:image"}
        )
        if album_art_link_element is None:
            print_error("No album art link for", link)
            return

        album_art_link = album_art_link_element.attrs["content"]

        print("Downloading image for", album_title)

        download_tasks.append(
            asyncio.create_task(
                download_album_art(
                    album_art_session, album_art_link, album_title, link[-17:]
                )
            )
        )


async def main():
    # Read in a file consisting of comma seperated URLs
    with open("links.txt") as link_file:
        links = link_file.read().split(", ")

    existing_images = set()
    for image_file in os.listdir("album_arts"):
        exif = piexif.load("album_arts/" + image_file)
        existing_images.add(exif["0th"][piexif.ImageIFD.ImageDescription])

    links_to_download = [
        link for link in links if link[-17:].encode() not in existing_images
    ]
    # Ensure folder to save images in is present
    os.makedirs("album_arts", exist_ok=True)

    download_tasks = []
    async with aiohttp.ClientSession() as youtube_music_session, aiohttp.ClientSession() as google_user_content_session:
        tasks = [
            request_album_page(
                youtube_music_session, google_user_content_session, link, download_tasks
            )
            for link in links_to_download
        ]
        await asyncio.gather(*tasks)
        await asyncio.gather(*download_tasks)


if __name__ == "__main__":
    asyncio.run(main())
