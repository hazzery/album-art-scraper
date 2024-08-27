import aiohttp
import asyncio
import os

from bs4 import BeautifulSoup


async def download_album_art(
    session: aiohttp.ClientSession, link: str, album_title: str
) -> None:
    """Asyncronously download the album art image from the specified link.

    :param session: The HTTP session to send the request with (reuses connection to improve speed).
    :param link: The URL to the image to download.
    :param album_title: The title of the album whose art to download.
    """
    async with session.get(link) as album_art_response:
        sanitised_album_title = album_title.replace("/", " ")
        filename = f"album_arts/{sanitised_album_title}.jpg"
        with open(filename, "wb") as album_art_file:
            album_art_file.write(await album_art_response.read())


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

        album_title = parsed_document.find("meta", attrs={"name": "title"}).attrs[
            "content"
        ]
        album_art_link = parsed_document.find(
            "meta", attrs={"property": "og:image"}
        ).attrs["content"]

        download_tasks.append(
            asyncio.create_task(
                download_album_art(album_art_session, album_art_link, album_title)
            )
        )


async def main():
    # Read in a file consisting of comma seperated URLs
    with open("links.txt") as link_file:
        links = link_file.read().split(", ")

    # Ensure folder to save images in is present
    os.makedirs("album_arts", exist_ok=True)

    download_tasks = []
    async with aiohttp.ClientSession() as youtube_music_session, aiohttp.ClientSession() as google_user_content_session:
        tasks = [
            request_album_page(
                youtube_music_session, google_user_content_session, link, download_tasks
            )
            for link in links
        ]
        await asyncio.gather(*tasks)
        await asyncio.gather(*download_tasks)


if __name__ == "__main__":
    asyncio.run(main())
