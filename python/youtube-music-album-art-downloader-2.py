import asyncio
import pathlib
import sys

import aiohttp
import piexif
from bs4 import BeautifulSoup, NavigableString


def print_error(*args: object, **kwargs) -> None:
    """Print to stderr instead of stdout.

    :param args: Values to be printed.
    :param kwargs: Key word arguments to pass to ``print``.
    """
    print(*args, file=sys.stderr, **kwargs)


async def download_album_art(
    session: aiohttp.ClientSession,
    link: str,
    album_title: str,
    youtube_album_code: str,
) -> None:
    """Asyncronously download the album art image from the specified link.

    :param session: The HTTP session to send the request with
        (reuses connection to improve speed).

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
            },
        )
        piexif.insert(exif_data, await album_art_response.read(), filename)


async def request_album_page(
    session: aiohttp.ClientSession,
    album_art_session: aiohttp.ClientSession,
    link: str,
    download_tasks: list[asyncio.Task],
) -> None:
    """Asyncronously fetch the YouTube Music album page at the specified URL.

    :param session: The HTTP session to send the request with
        (reuses connection to improve speed).

    :param album_art_session: The HTTP session to send album art image request
        with (reuses connection to improve speed).

    :param link: The URL to the YouTube Music album page.
    :param download_tasks: A list of tasks to append the image download task to.
    """
    async with session.get(link) as album_page_response:
        parsed_document = BeautifulSoup(
            await album_page_response.text(),
            features="html.parser",
        )

        album_title_element = parsed_document.find("meta", attrs={"name": "title"})
        if album_title_element is None or isinstance(
            album_title_element,
            NavigableString,
        ):
            print_error("No album title for", link)
            return

        album_title = album_title_element.attrs["content"]

        album_art_link_element = parsed_document.find(
            "meta",
            attrs={"property": "og:image"},
        )
        if album_art_link_element is None or isinstance(
            album_art_link_element,
            NavigableString,
        ):
            print_error("No album art link for", link)
            return

        album_art_link = album_art_link_element.attrs["content"]

        print("Downloading image for", album_title)

        download_tasks.append(
            asyncio.create_task(
                download_album_art(
                    album_art_session,
                    album_art_link,
                    album_title,
                    link[-17:],
                ),
            ),
        )


async def run_downloads(links_to_download: list[str]) -> None:
    """Download album art images for each album link in ``links_to_download``.

    :param links_to_download: A list of links to YouTube Music album pages.
    """
    download_tasks = []
    async with (
        aiohttp.ClientSession() as youtube_music_session,
        aiohttp.ClientSession() as google_user_content_session,
    ):
        tasks = [
            request_album_page(
                youtube_music_session,
                google_user_content_session,
                link,
                download_tasks,
            )
            for link in links_to_download
        ]
        await asyncio.gather(*tasks)
        await asyncio.gather(*download_tasks)


def get_codes_of_existing_album_art(album_art_directory_name: str) -> set[str]:
    """Create a set of all existing album art images.

    :param album_art_directory_name: The name of the directory to find existing
        images in.

    :returns: A set of the album codes for each existing image.

    :raises OSError: If ``album_art_directory_name`` points to a file.
    """
    album_art_directory = pathlib.Path(album_art_directory_name)
    if not album_art_directory.exists():
        album_art_directory.mkdir()
        return set()

    if not album_art_directory.is_dir():
        message = (
            f"Unable to read album art images from {album_art_directory_name}"
            " as it is not a directory."
        )
        raise OSError(message)

    existing_images = set()
    for image_file in album_art_directory.iterdir():
        exif = piexif.load(str(image_file))
        existing_images.add(exif["0th"][piexif.ImageIFD.ImageDescription])

    return existing_images


def get_all_links(links_file_name: str) -> list[str]:
    """Read in all links from the links file.

    :param links_file_name: The name of the file to read album links from.

    :returns: A list of all album links in the links file.

    :raises OSError: If ``links_file_name`` points to a directory.
    """
    links_file = pathlib.Path(links_file_name)
    if not links_file.exists():
        return []

    if not links_file.is_file():
        message = f"Unable to read links from {links_file_name} as it is not a file."
        raise OSError(message)

    return links_file.read_text().split(", ")


def get_links_to_download(
    links_file_name: str,
    album_art_directory_name: str,
) -> list[str]:
    """Read in all links from file and check for ones already present.

    :param links_file_name: The name of the file to find links in.

    :param album_art_directory_name: The name of the directory to find
        existing images in.
    """
    all_links = get_all_links(links_file_name)
    existing_album_codes = get_codes_of_existing_album_art(album_art_directory_name)

    return [
        link for link in all_links if link[-17:].encode() not in existing_album_codes
    ]


def main() -> None:
    """Figure out which imgages already exist and download new ones."""
    links_to_download = get_links_to_download("links.txt", "album_arts")

    asyncio.run(run_downloads(links_to_download))


if __name__ == "__main__":
    main()
